package generator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"log"
	"slices"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/snonky/astpos/astpos"
	"golang.org/x/tools/go/ast/astutil"
)

var (
	ErrEmbeddedField = errors.New("Generation failed because the `%v` template struct contains an anonymous embedded field.")
)

func Generate(templateParser *Parser, savePath, packageName string) ([]byte, error) {
	if !validatePackageName(packageName) {
		errMsg := fmt.Sprintf("The package name %v is not valid.", packageName)
		return nil, errors.New(errMsg)
	}

	if err := loadPBInfo(); err != nil {
		return nil, err
	}

	decls, err := proxiesFromGoTemplate(templateParser)
	if err != nil {
		return nil, err
	}

	f := wrapGeneratedDeclarations(decls, packageName)

	f, fset := astpos.RewritePositions(f)
	sourceCode, err := printAST(f, fset, savePath)
	if err != nil {
		return nil, err
	}

	err = checkPbShadows(sourceCode)
	if err != nil {
		return nil, err
	}

	return sourceCode, nil
}

func checkPbShadows(sourceCode []byte) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "shadowcheck.go", sourceCode, parser.SkipObjectResolution)
	if err != nil {
		return err
	}

	conf := types.Config{Importer: &Importer{}}
	pkg, err := conf.Check("x", fset, []*ast.File{f}, nil)
	if pkg == nil {
		// Do not check error here because type errors can happen
		// with only a single file being checked w/o dependencies.
		// We only want the scope names for the shadow check.
		return err
	}

	scope := pkg.Scope()
	names := scope.Names()
	allShadows := make([]string, 0)
	for _, name := range names {
		obj := scope.Lookup(name)
		proxyType, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		_, ok = proxyType.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		_, shadows := pbInfo.shadowsRecord(proxyType)
		allShadows = append(allShadows, shadows...)
	}

	if len(allShadows) > 0 {
		errMsg := fmt.Sprintf(`Can not generate proxy code because some of the generated names shadow names from PocketBase's core.Record struct. This prevents the internals of PocketBase to safely handle data.
Try renaming fields/methods in the template to escape the shadowing. Don't forget to use the '// schema-name:' comment when renaming fields.
Additionally make sure that all the system fields in your template are marked by the '// system:' comment and do not change the generated system comments.
The shadowed names are: %v`, allShadows)
		return errors.New(errMsg)
	}

	return nil
}

// Takes a parsed template file and creates a proxy for every
// struct that is found in it.
// Each proxy has a getter/setter for each field
// in the template struct.
// Fields with an unknown type are ignored with
// a warning.
func proxiesFromGoTemplate(p *Parser) ([]ast.Decl, error) {
	proxyMethods, err := createProxyMethods(p)
	if err != nil {
		return nil, err
	}

	decls := make([]ast.Decl, 0, 25)
	var fset = token.NewFileSet()
	for _, s := range p.structSpecs {
		structName := s.Name.Name
		fields := p.structFields[structName]

		startComment := &ast.Comment{
			Slash: token.Pos(fset.Base()),
			Text:  fmt.Sprintf("// %s ------------------------------------------------------------------------------------------------------------------ ", structName),
		}
		startCommentGroup := &ast.CommentGroup{
			List: []*ast.Comment{startComment},
		}

		decls = append(decls, createSelectTypes(fields)...)

		var finalDoc *ast.CommentGroup
		if s.Doc != nil {
			finalDoc = &ast.CommentGroup{
				List: append(startCommentGroup.List, s.Doc.List...),
			}
		} else {
			finalDoc = startCommentGroup
		}

		decls = append(decls, newProxyDecl(structName, finalDoc))

		methods := proxyMethods[structName]
		decls = append(decls, methods...)

		nameGetter := p.createCollectionNameGetter(structName)
		if nameGetter != nil {
			decls = append(decls, nameGetter)
		}

		getters, err := createFuncs(fields, newGetterDecl)
		if err != nil {
			return nil, err
		}
		setters, err := createFuncs(fields, newSetterDecl)
		if err != nil {
			return nil, err
		}
		for i, getter := range getters {
			if getter == nil {
				continue
			}
			decls = append(decls, getter, setters[i])
		}
	}

	return decls, nil
}

type Parser struct {
	sourceCode []byte

	Fset *token.FileSet
	fAst *ast.File

	structSpecs     []*ast.TypeSpec
	structNames     map[string]*ast.TypeSpec
	structFields    map[string][]*Field
	structMethods   map[string][]*ast.FuncDecl
	collectionNames map[string]string

	// Tracks new identifier names that the parser finds from
	// template comments
	newNames map[string]any

	// Tracks the declarations of select-typing related
	// names to prevent duplication
	selectTypeToOptions  map[string][]string
	selectTypeToVarNames map[string][]string
}

func NewTemplateParser(sourceCode []byte) (*Parser, error) {
	p := &Parser{
		sourceCode:           sourceCode,
		newNames:             map[string]any{},
		selectTypeToOptions:  map[string][]string{},
		selectTypeToVarNames: map[string][]string{},
	}
	if err := p.parseFile(); err != nil {
		return nil, err
	}

	p.collectStructSpecs()
	if err := p.collectStructFields(); err != nil {
		return nil, err
	}
	p.collectStructMethods()
	p.findCollectionNames()

	return p, nil
}

func (p *Parser) parseFile() error {
	p.Fset = token.NewFileSet()

	opts := parser.SkipObjectResolution |
		parser.ParseComments
	f, err := parser.ParseFile(p.Fset, "x.go", p.sourceCode, opts)
	if err != nil {
		return err
	}

	p.fAst = f
	return nil
}

func (p *Parser) collectStructSpecs() {
	specs := make([]*ast.TypeSpec, 0, 16)
	names := make(map[string]*ast.TypeSpec)

	ast.Inspect(p.fAst, func(n ast.Node) bool {
		structSpec := structSpec(n)
		if structSpec != nil {
			specs = append(specs, structSpec)
			names[structSpec.Name.Name] = structSpec
			return false
		}
		return true
	})

	p.structSpecs = specs
	p.structNames = names
}

func (p *Parser) collectStructFields() error {
	p.structFields = make(map[string][]*Field)

	for _, s := range p.structSpecs {
		structName := s.Name.Name
		astFields := s.Type.(*ast.StructType).Fields.List
		fields := make([]*Field, 0, len(astFields))

		for _, f := range astFields {
			fs, err := p.newFieldsFromAST(structName, f)
			if err != nil {
				return err
			}
			fields = append(fields, fs...)
		}

		p.structFields[structName] = fields
	}
	return nil
}

func (p *Parser) collectStructMethods() {
	funcs := make(map[string][]*ast.FuncDecl)

	for _, decl := range p.fAst.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		recv := funcDecl.Recv
		if recv == nil {
			continue
		}

		recvType := baseType(recv.List[0].Type)
		recvName, _ := nodeString(recvType)

		_, ok = funcs[recvName]
		if !ok {
			funcs[recvName] = make([]*ast.FuncDecl, 0)
		}
		funcs[recvName] = append(funcs[recvName], funcDecl)
	}

	p.structMethods = funcs
}

func (p *Parser) findCollectionNames() {
	p.collectionNames = make(map[string]string)

	for structName, fields := range p.structFields {
		if len(fields) == 0 {
			continue
		}
		firstField := fields[0].astOriginal
		cName := p.parseCollectionNameComment(firstField)
		if cName != "" {
			p.collectionNames[structName] = cName
		}
	}
}

func (p *Parser) newFieldsFromAST(structName string, field *ast.Field) ([]*Field, error) {
	if len(field.Names) == 0 {
		return nil, ErrEmbeddedField
	}

	selectTypeName, selectOptions, selectVarNames, err := p.parseSelectTypeComment(field)
	if err != nil {
		return nil, err
	}
	schemaName, err := p.parseAlternativeSchemaName(field)
	if err != nil {
		return nil, err
	}
	if schemaName == "" {
		schemaName = field.Names[0].Name
	}

	systemFieldName, err := p.parseSystemFieldNameComment(field)
	if err != nil {
		return nil, err
	}

	fields := make([]*Field, len(field.Names))
	for i, n := range field.Names {
		fieldName := n.Name
		field := newField(
			structName,
			fieldName,
			schemaName,
			systemFieldName,
			field.Type,
			selectTypeName,
			selectOptions,
			selectVarNames,
			p.structNames,
			field,
			p,
		)
		fields[i] = field
	}

	return fields, nil
}

var selectTypeComment = "// select:"

func (p *Parser) parseSelectTypeComment(field *ast.Field) (string, []string, []string, error) {
	if field.Doc == nil || len(field.Doc.List) == 0 {
		return "", nil, nil, nil
	}

	comment := ""
	var astComment *ast.Comment
	for _, c := range field.Doc.List {
		if len(c.Text) >= len(selectTypeComment) && c.Text[:len(selectTypeComment)] == selectTypeComment {
			comment = c.Text
			astComment = c
			break
		}
	}
	if comment == "" {
		return "", nil, nil, nil
	}

	typeName, err := nodeString(baseType(field.Type))
	if err != nil {
		return "", nil, nil, err
	}
	if typeName != "int" {
		pos := p.Fset.Position(astComment.Slash)
		err = p.createError("Cannot have // select: comment on field of type other than int or []int", pos, nil)
		return "", nil, nil, err
	}

	if len(field.Names) > 1 {
		pos := p.Fset.Position(astComment.Slash)
		errMsg := fmt.Sprintf("The // select: comment can only be used on fields with one identifier. Found %v.", len(field.Names))
		err = p.createError(errMsg, pos, nil)
		return "", nil, nil, err
	}

	comment = strings.TrimSpace(comment[len(selectTypeComment):])

	typeName, selectOptions, selectVarNames, err := p.parseSelectType(astComment.Slash, comment)
	if err != nil {
		return "", nil, nil, err
	}
	typeName, selectOptions, selectVarNames = p.validateSelectType(astComment.Slash, typeName, selectOptions, selectVarNames)

	return typeName, selectOptions, selectVarNames, nil
}

// Parses a string of the form 'TypeName(option1, option2, ...)' or
// 'TypeName(option1, option2, ...)[VarName1, VarName2, ...]
// Returns the [TypeName], the [option1, option1, ...] list and the [VarName1, VarName2, ...] list.
// If the var name list is omitted the option names are reused as the var names.
func (p *Parser) parseSelectType(commentPos token.Pos, typeStr string) (string, []string, []string, error) {
	parsed, err := parser.ParseExpr(typeStr)
	if err != nil {
		parserErr := err.(scanner.ErrorList)[0]
		pos := p.Fset.Position(commentPos)
		return "", nil, nil, p.createError(parserErr.Msg, pos, parserErr)
	}

	withVarNames, err := p.checkBrackets(commentPos, parsed)
	if err != nil {
		return "", nil, nil, err
	}

	typeName := ""
	selectOptions := make([]string, 0, 8)
	selectVarNames := make([]string, 0, 8)
	identFinder := func(c *astutil.Cursor) bool {
		ident, ok := c.Node().(*ast.Ident)
		if !ok {
			return true
		}
		switch c.Name() {
		case "Fun":
			typeName = ident.Name
		case "Args":
			selectOption, _ := trimUnderscore(ident.Name)
			selectOptions = append(selectOptions, selectOption)
			if !withVarNames {
				varName := strcase.ToCamel(ident.Name)
				selectVarNames = append(selectVarNames, varName)
			}
		case "Index":
			fallthrough
		case "Indices":
			varName := strcase.ToCamel(ident.Name)
			selectVarNames = append(selectVarNames, varName)
		}
		return true
	}
	astutil.Apply(parsed, identFinder, nil)

	if typeName == "" || len(selectOptions) == 0 {
		pos := p.Fset.Position(commentPos)
		err = p.createError("Malformed // select: comment. Example usage: // select: TypeName(option1, option2)[VarName1, VarName2]", pos, nil)
		if err != nil {
			return "", nil, nil, err
		}
	}

	if len(selectOptions) != len(selectVarNames) {
		pos := p.Fset.Position(commentPos)
		errMsg := fmt.Sprintf(
			"Unequal number of select options and variable names in // select: comment. Found %v options and %v names",
			len(selectOptions),
			len(selectVarNames),
		)
		err = p.createError(errMsg, pos, nil)
		return "", nil, nil, err
	}

	return typeName, selectOptions, selectVarNames, nil
}

// Checks if the // select: comment only has () or also includes []
// Errors if neither are found. Returns true when the [] are present.
func (p *Parser) checkBrackets(commentPos token.Pos, parsedComment ast.Node) (bool, error) {
	var indexExpr ast.Expr
	var callExpr ast.Expr

	switch n := parsedComment.(type) {
	case *ast.IndexExpr:
		indexExpr = n
		callExpr = n.X
	case *ast.IndexListExpr:
		indexExpr = n
		callExpr = n.X
	case *ast.CallExpr:
		callExpr = n
	}

	withVarNames := indexExpr != nil
	withOpts := callExpr != nil
	if !withVarNames && !withOpts {
		pos := p.Fset.Position(commentPos)
		err := p.createError("Malformed // select: comment. Example usage: // select: TypeName(option1, option2)[VarName1, VarName2]", pos, nil)
		return false, err
	}

	return withVarNames, nil
}

func (p *Parser) validateSelectType(commentPos token.Pos, typeName string, selectOptions, selectVarNames []string) (string, []string, []string) {
	origName := typeName
	_, isDuplicate := p.newNames[typeName]

	if isDuplicate {
		otherOpts := p.selectTypeToOptions[typeName]
		otherVars := p.selectTypeToVarNames[typeName]
		if slices.Equal(selectOptions, otherOpts) && slices.Equal(selectVarNames, otherVars) {
			// Another field already defined the same select type. Reuse.
			return typeName, []string{}, []string{}
		} else {
			// Another field has a duplicate name but different select options. Rename this one.
			typeName = rename(typeName, p.newNames)

			pos := p.Fset.Position(commentPos)
			warnMsg := fmt.Sprintf("Found a duplicate select type name: %v. Renaming to %v", origName, typeName)
			p.logWarning(warnMsg, pos, nil)
		}
	}

	p.newNames[typeName] = struct{}{}
	p.selectTypeToOptions[typeName] = selectOptions
	p.selectTypeToVarNames[typeName] = selectVarNames

	selectVarNames = p.checkSelectVarNameDuplicates(commentPos, selectVarNames)

	return typeName, selectOptions, selectVarNames
}

func (p *Parser) checkSelectVarNameDuplicates(commentPos token.Pos, selectVarNames []string) []string {
	checkedNames := make([]string, len(selectVarNames))

	for i, name := range selectVarNames {
		_, isDuplicate := p.newNames[name]

		if isDuplicate {
			name = rename(name, p.newNames)

			pos := p.Fset.Position(commentPos)
			warnMsg := fmt.Sprintf(
				"Found a duplicate select variable name. Renaming to %v.", name,
			)
			p.logWarning(warnMsg, pos, nil)
		}

		p.newNames[name] = struct{}{}

		checkedNames[i] = name
	}

	return checkedNames
}

var schemaNameComment = "// schema-name:"

func (p *Parser) parseAlternativeSchemaName(field *ast.Field) (string, error) {
	if field.Doc == nil || len(field.Doc.List) == 0 {
		return p.trailingUnderscoreName(field)
	}

	comment := ""
	var astComment *ast.Comment
	for _, c := range field.Doc.List {
		if len(c.Text) >= len(schemaNameComment) && c.Text[:len(schemaNameComment)] == schemaNameComment {
			comment = c.Text
			astComment = c
			break
		}
	}
	if comment == "" {
		return p.trailingUnderscoreName(field)
	}

	if len(field.Names) > 1 {
		pos := p.Fset.Position(astComment.Slash)
		errMsg := fmt.Sprintf("The // schema-name: comment can only be used on fields with one identifier. Found %v.", len(field.Names))
		return "", p.createError(errMsg, pos, nil)
	}

	schemaname := strings.TrimSpace(comment[len(schemaNameComment):])
	return schemaname, nil
}

var systemFieldComment = "// system:"

func (p *Parser) parseSystemFieldNameComment(field *ast.Field) (string, error) {
	if field.Doc == nil || len(field.Doc.List) == 0 {
		return "", nil
	}

	comment := ""
	var astComment *ast.Comment
	for _, c := range field.Doc.List {
		if len(c.Text) >= len(systemFieldComment) && c.Text[:len(systemFieldComment)] == systemFieldComment {
			comment = c.Text
			astComment = c
			break
		}
	}
	if comment == "" {
		return "", nil
	}

	if len(field.Names) > 1 {
		pos := p.Fset.Position(astComment.Slash)
		errMsg := "The // system: comment can only be used on fields with one identifier and should not be changed from its generated form."
		return "", p.createError(errMsg, pos, nil)
	}

	systemFieldName := strings.TrimSpace(comment[len(systemFieldComment):])
	return systemFieldName, nil
}

var collectionNameComment = "// collection-name:"

func (p *Parser) parseCollectionNameComment(field *ast.Field) string {
	if field.Doc == nil || len(field.Doc.List) == 0 {
		return ""
	}

	collectionName := ""
	for _, c := range field.Doc.List {
		if len(c.Text) >= len(collectionNameComment) && c.Text[:len(collectionNameComment)] == collectionNameComment {
			collectionName = strings.TrimSpace(c.Text[len(collectionNameComment):])
			break
		}
	}

	return collectionName
}

// A trailing underscore signals an identifier that could otherwise
// not be used because it is a reserved go keyword like "type" or "func".
// This function returns the identifier name without the trailing underscore.
// If no trailing underscore is present, an empty string is returned.
func (p *Parser) trailingUnderscoreName(field *ast.Field) (string, error) {
	tuName := ""
	for _, n := range field.Names {
		trimmed, ok := trimUnderscore(n.Name)
		if ok {
			tuName = trimmed
			break
		}
	}
	if tuName != "" && len(field.Names) > 1 {
		pos := p.Fset.Position(field.Pos())
		errMsg := fmt.Sprintf("Trailing underscore identifiers can only be used on fields with one identifier. Found %v.", len(field.Names))
		return "", p.createError(errMsg, pos, nil)
	}
	return tuName, nil
}

func (p *Parser) createError(msg string, pos token.Position, origErr *scanner.Error) error {
	if origErr != nil {
		pos.Column = origErr.Pos.Column
	}
	errMsg := fmt.Sprintf("Error: %v: %v", pos, msg)
	return errors.New(errMsg)
}

func (p *Parser) logWarning(msg string, pos token.Position, origErr *scanner.Error) {
	if origErr != nil {
		pos.Column = origErr.Pos.Column
	}
	log.Printf("Warning: %v: %v", pos, msg)
}

func rename(name string, existingNames map[string]any) string {
	newName := name
	for i := 2; ; i += 1 {
		newName = fmt.Sprintf("%v%v", name, i)
		_, isDuplicate := existingNames[newName]
		if !isDuplicate {
			break
		}
	}
	return newName
}

func createFuncs(fields []*Field, declare func(f *Field) (*ast.FuncDecl, error)) ([]*ast.FuncDecl, error) {
	decls := make([]*ast.FuncDecl, 0, len(fields))
	for _, f := range fields {
		if f.systemFieldName == "" {
			decl, err := declare(f)
			if err != nil {
				return nil, err
			}
			decls = append(decls, decl)
		}
	}

	return decls, nil
}

func createSelectTypes(fields []*Field) []ast.Decl {
	decls := make([]ast.Decl, 0, 10)
	for _, f := range fields {
		if len(f.selectOptions) == 0 {
			continue
		}
		decls = append(
			decls,
			newSelectTypeDecl(f.selectTypeName),
			newSelectConstDecl(f),
			newSelectMapDecl(f, true),
			newSelectMapDecl(f, false),
		)
	}

	return decls
}

func (p *Parser) createCollectionNameGetter(structName string) *ast.FuncDecl {
	collectionName := p.collectionNames[structName]
	if collectionName == "" {
		warnMsg := fmt.Sprintf(
			"Warning: The `%v` template struct does not have a '// collection-name:' comment on its first field. Skipping generation of the CollectionName() method.",
			structName,
		)
		log.Println(warnMsg)
		return nil
	}

	getterDecl, _ := newCollectionNameGetter("CollectionName", structName, collectionName)

	return getterDecl
}

// Returns a *ast.TypeSpec if it specifies a struct.
// Otherwise nil
func structSpec(n ast.Node) *ast.TypeSpec {
	structDecl, ok := n.(*ast.GenDecl)
	if !ok {
		return nil
	}
	structSpec, ok := structDecl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return nil
	}
	_, ok = structSpec.Type.(*ast.StructType)
	if !ok {
		return nil
	}
	structSpec.Doc = structDecl.Doc
	return structSpec
}

// Removes one trailing underscore from a string
// if present and returns it with true.
// Otherwise returns s and false.
func trimUnderscore(s string) (string, bool) {
	if len(s) > 1 && s[len(s)-1] == '_' {
		return s[:len(s)-1], true
	}
	return s, false
}
