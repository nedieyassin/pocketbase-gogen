package generator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/go-toolsmith/astcopy"
	"github.com/iancoleman/strcase"
	"golang.org/x/tools/go/ast/astutil"
)

var (
	structTemplate *ast.GenDecl

	getterTemplate,
	relationGetterTemplate,
	multiRelationGetterTemplate,
	selectGetterTemplate,
	multiSelectGetterTemplate,

	setterTemplate,
	relationSetterTemplate,
	multiRelationSetterTemplate,
	selectSetterTemplate,
	multiSelectSetterTemplate *ast.FuncDecl

	collectionNameGetter *ast.FuncDecl

	primitiveGetters map[string]string
)

type RelationType int

const (
	singleRel RelationType = iota
	multiRel
)

type Field struct {
	structName, fieldName, schemaName string
	fieldType                         ast.Expr

	// Only set for system fields
	systemFieldName string

	// Only set for select type fields
	selectTypeName string
	selectOptions  []string
	selectVarNames []string

	allProxyNames map[string]*ast.TypeSpec

	astOriginal *ast.Field
	parser      *Parser
}

func newField(
	structName,
	fieldName,
	schemaName,
	systemFieldName string,
	fieldType ast.Expr,
	selectTypeName string,
	selectOptions []string,
	selectVarNames []string,
	allProxyNames map[string]*ast.TypeSpec,
	astOriginal *ast.Field,
	parser *Parser,
) *Field {
	return &Field{
		structName:      structName,
		fieldName:       fieldName,
		schemaName:      schemaName,
		systemFieldName: systemFieldName,
		fieldType:       fieldType,
		selectTypeName:  selectTypeName,
		selectOptions:   selectOptions,
		selectVarNames:  selectVarNames,
		allProxyNames:   allProxyNames,
		astOriginal:     astOriginal,
		parser:          parser,
	}
}

func loadTemplateASTs() error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, ".", proxyTemplateCode, parser.SkipObjectResolution)
	if err != nil {
		return err
	}

	structTemplate = f.Decls[0].(*ast.GenDecl)

	getterTemplate = f.Decls[1].(*ast.FuncDecl)
	relationGetterTemplate = f.Decls[2].(*ast.FuncDecl)
	multiRelationGetterTemplate = f.Decls[3].(*ast.FuncDecl)
	selectGetterTemplate = f.Decls[4].(*ast.FuncDecl)
	multiSelectGetterTemplate = f.Decls[5].(*ast.FuncDecl)

	setterTemplate = f.Decls[6].(*ast.FuncDecl)
	relationSetterTemplate = f.Decls[7].(*ast.FuncDecl)
	multiRelationSetterTemplate = f.Decls[8].(*ast.FuncDecl)
	selectSetterTemplate = f.Decls[9].(*ast.FuncDecl)
	multiSelectSetterTemplate = f.Decls[10].(*ast.FuncDecl)

	collectionNameGetter = f.Decls[11].(*ast.FuncDecl)

	return nil
}

func loadPBInfo() error {
	info, err := newPocketBaseInfo()
	if err != nil {
		return err
	}
	pbInfo = info
	primitiveGetters = info.recordGetters
	return nil
}

func newProxyDecl(name string, doc *ast.CommentGroup) *ast.GenDecl {
	proxy := astcopy.GenDecl(structTemplate)
	proxy.Specs[0].(*ast.TypeSpec).Name.Name = name
	proxy.Doc = doc

	return proxy
}

func newGetterDecl(field *Field) (*ast.FuncDecl, error) {
	if field.selectTypeName != "" {
		return newSelectGetterDecl(field)
	}

	returnTypeName, err := nodeString(field.fieldType)
	if err != nil {
		return nil, err
	}
	getterName, ok := primitiveGetters[returnTypeName]
	if !ok {
		return newRelGetterDecl(field)
	}

	return newPrimitiveGetterDecl(field, getterName)
}

func newPrimitiveGetterDecl(field *Field, recordGetterName string) (*ast.FuncDecl, error) {
	decl := astcopy.FuncDecl(getterTemplate)

	err := adaptFuncTemplate(
		decl,
		field.structName,
		getterName(field.fieldName),
		recordGetterName,
		field.fieldName,
		field.schemaName,
		field.fieldType,
	)
	if err != nil {
		return nil, err
	}

	return decl, nil
}

func newRelGetterDecl(field *Field) (*ast.FuncDecl, error) {
	fieldType := field.fieldType
	fieldName := field.fieldName

	relType := baseType(fieldType)
	relTypeName, err := nodeString(relType)
	if err != nil {
		return nil, err
	}
	_, ok := field.allProxyNames[relTypeName]
	if !ok {
		returnTypeName, err := nodeString(fieldType)
		if err != nil {
			return nil, err
		}
		pos := field.parser.Fset.Position(field.astOriginal.Pos())
		errMsg := fmt.Sprintf(
			"Unable to generate relation getter/setter for field `%v` of type %v. All relation fields must have the related type also be a proxy.",
			fieldName, returnTypeName,
		)
		err = field.parser.createError(errMsg, pos, nil)
		return nil, err
	}

	var decl *ast.FuncDecl
	switch relationType(fieldType) {
	case singleRel:
		decl = astcopy.FuncDecl(relationGetterTemplate)
	case multiRel:
		decl = astcopy.FuncDecl(multiRelationGetterTemplate)
	}

	err = adaptFuncTemplate(
		decl,
		field.structName,
		getterName(fieldName),
		"",
		fieldName,
		field.schemaName,
		relType,
	)
	if err != nil {
		return nil, err
	}

	return decl, nil
}

func newSelectGetterDecl(field *Field) (*ast.FuncDecl, error) {
	var decl *ast.FuncDecl
	if relationType(field.fieldType) == singleRel {
		decl = astcopy.FuncDecl(selectGetterTemplate)
	} else {
		decl = astcopy.FuncDecl(multiSelectGetterTemplate)
	}

	err := adaptFuncTemplate(
		decl,
		field.structName,
		getterName(field.fieldName),
		"",
		field.fieldName,
		field.schemaName,
		&ast.Ident{Name: field.selectTypeName},
	)
	if err != nil {
		return nil, err
	}

	return decl, nil
}

func newCollectionNameGetter(getterName, structName, collectionName string) (*ast.FuncDecl, error) {
	decl := astcopy.FuncDecl(collectionNameGetter)

	err := adaptFuncTemplate(
		decl,
		structName,
		getterName,
		"",
		"",
		collectionName,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return decl, nil
}

func newSetterDecl(field *Field) (*ast.FuncDecl, error) {
	fieldName := field.fieldName
	fieldType := field.fieldType

	var decl *ast.FuncDecl

	switch field.selectTypeName {
	case "":
		returnTypeName, err := nodeString(fieldType)
		if err != nil {
			return nil, err
		}
		_, ok := primitiveGetters[returnTypeName]
		if !ok {
			return newRelSetterDecl(field)
		}
		decl = astcopy.FuncDecl(setterTemplate)
	default:
		fieldType = &ast.Ident{Name: field.selectTypeName}
		switch relationType(field.fieldType) {
		case singleRel:
			decl = astcopy.FuncDecl(selectSetterTemplate)
		case multiRel:
			decl = astcopy.FuncDecl(multiSelectSetterTemplate)
		}
	}

	err := adaptFuncTemplate(
		decl,
		field.structName,
		setterName(fieldName),
		"",
		fieldName,
		field.schemaName,
		fieldType,
	)
	if err != nil {
		return nil, err
	}

	return decl, nil
}

func newRelSetterDecl(field *Field) (*ast.FuncDecl, error) {
	fieldType := field.fieldType
	fieldName := field.fieldName

	relType := baseType(fieldType)
	relTypeName, err := nodeString(relType)
	if err != nil {
		return nil, err
	}
	_, ok := field.allProxyNames[relTypeName]
	if !ok {
		// The warning will be logged by newRelGetterDecl
		errMsg := fmt.Sprintf("Could not identify the relation field type `%v` on the `%v.%v` field", relTypeName, field.structName, fieldName)
		return nil, errors.New(errMsg)
	}

	var decl *ast.FuncDecl
	switch relationType(fieldType) {
	case singleRel:
		decl = astcopy.FuncDecl(relationSetterTemplate)
	case multiRel:
		decl = astcopy.FuncDecl(multiRelationSetterTemplate)
	}

	err = adaptFuncTemplate(
		decl,
		field.structName,
		setterName(fieldName),
		"",
		fieldName,
		field.schemaName,
		relType,
	)
	if err != nil {
		return nil, err
	}

	return decl, nil
}

// Scans a function declaration template for a set of pre-defined
// identifiers and literals and replaces them with the given values
func adaptFuncTemplate(
	template *ast.FuncDecl,
	receiverName,
	funcName,
	getterFuncName,
	fieldName,
	schemaFieldName string,
	fieldType ast.Expr,
) error {
	var adapterErr error
	adapter := func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.Ident:
			switch n.Name {
			case "StructName":
				n.Name = receiverName
			case "FuncName":
				n.Name = funcName
			case "GetterFuncName":
				n.Name = getterFuncName
			case "fieldName":
				n.Name = strcase.ToLowerCamel(fieldName)
				if fieldName[len(fieldName)-1] == '_' {
					n.Name += "_"
				}
			case "selectNameMap":
				baseTypeName, err := nodeString(baseType(fieldType))
				if err != nil {
					adapterErr = err
					return false
				}
				n.Name = selectNameMapName(baseTypeName)
			case "selectIotaMap":
				baseTypeName, err := nodeString(baseType(fieldType))
				if err != nil {
					adapterErr = err
					return false
				}
				n.Name = selectIotaMapName(baseTypeName)
			case "FieldType":
				c.Replace(fieldType)
			}
		case *ast.BasicLit:
			if n.Value == "\"key\"" {
				keyLiteral := fmt.Sprintf("\"%v\"", schemaFieldName)
				n.Value = keyLiteral
			}
		}
		return true
	}

	astutil.Apply(template, adapter, nil)

	return adapterErr
}

func newSelectTypeDecl(name string) *ast.GenDecl {
	spec := &ast.TypeSpec{
		Name: &ast.Ident{Name: name},
		Type: &ast.Ident{Name: "int"},
	}
	decl := &ast.GenDecl{
		Specs: []ast.Spec{spec},
		Tok:   token.TYPE,
	}
	return decl
}

func newSelectConstDecl(field *Field) *ast.GenDecl {
	typeName := field.selectTypeName
	varNames := field.selectVarNames

	if len(varNames) == 0 {
		return nil
	}

	specs := make([]ast.Spec, len(varNames))

	valIdents := make([]*ast.Ident, len(varNames))
	for i := range len(varNames) {
		valIdents[i] = &ast.Ident{
			Name: varNames[i],
		}
	}

	specs[0] = &ast.ValueSpec{
		Names:  valIdents[:1],
		Type:   &ast.Ident{Name: typeName},
		Values: []ast.Expr{&ast.Ident{Name: "iota"}},
	}

	for i := 1; i < len(varNames); i += 1 {
		specs[i] = &ast.ValueSpec{
			Names: valIdents[i : i+1],
		}
	}

	decl := &ast.GenDecl{Specs: specs, Tok: token.CONST}

	return decl
}

func newSelectMapDecl(field *Field, invertMapping bool) *ast.GenDecl {
	typeName := field.selectTypeName
	options := field.selectOptions

	if len(options) == 0 {
		return nil
	}

	key, val := typeName, "string"
	if invertMapping {
		key, val = val, key
	}

	mapType := &ast.MapType{
		Key:   &ast.Ident{Name: key},
		Value: &ast.Ident{Name: val},
	}

	mapElems := make([]ast.Expr, len(options))
	for i, e := range options {
		nameLit := &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("\"%v\"", e),
		}
		indexIdent := &ast.Ident{Name: fmt.Sprintf("%v", i)}
		var key, val ast.Expr = indexIdent, nameLit
		if invertMapping {
			key, val = val, key
		}
		mapElems[i] = &ast.KeyValueExpr{Key: key, Value: val}
	}

	mapLiteral := &ast.CompositeLit{
		Type: mapType,
		Elts: mapElems,
	}

	varName := ""
	if invertMapping {
		varName = selectNameMapName(typeName)
	} else {
		varName = selectIotaMapName(typeName)
	}
	spec := &ast.ValueSpec{
		Names:  []*ast.Ident{{Name: varName}},
		Values: []ast.Expr{mapLiteral},
	}

	decl := &ast.GenDecl{Specs: []ast.Spec{spec}, Tok: token.VAR}

	return decl
}

func selectNameMapName(typeName string) string {
	// zz to keep it at the bottom of intellisense
	return fmt.Sprintf("zz%vSelectNameMap", typeName)
}

func selectIotaMapName(typeName string) string {
	// zz to keep it at the bottom of intellisense
	return fmt.Sprintf("zz%vSelectIotaMap", typeName)
}

// Returns the base type of a type expression
// Examples: *int -> int, []string -> string, []*MyStruct -> MyStruct
func baseType(t ast.Expr) ast.Expr {
	var base *ast.Ident
	ast.Inspect(t, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if ok {
			base = ident
		}
		return !ok
	})
	return base
}

// Returns multiRel if the type expression t is an
// array type and singleRel otherwise
func relationType(t ast.Expr) RelationType {
	relType := singleRel
	ast.Inspect(t, func(n ast.Node) bool {
		_, ok := n.(*ast.ArrayType)
		if ok {
			relType = multiRel
		}
		return !ok
	})
	return relType
}
