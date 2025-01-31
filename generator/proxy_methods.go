package generator

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"log"

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/go/ast/astutil"
)

func createProxyMethods(parser *Parser) map[string][]ast.Decl {
	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	_, err := conf.Check("template", parser.Fset, []*ast.File{parser.fAst}, info)
	if err != nil {
		log.Fatal(err)
	}

	decls := make(map[string][]ast.Decl)
	for _, s := range parser.structSpecs {
		structName := s.Name.Name
		methods := parser.structMethods[structName]
		fields := parser.structFields[structName]

		proxyMethods := make([]ast.Decl, len(methods))
		for i, m := range methods {
			newMethodProxifier(m, fields, info, parser.structNames).proxify()
			proxyMethods[i] = m
		}

		decls[structName] = proxyMethods
	}

	return decls
}

type methodProxifier struct {
	method           *ast.FuncDecl
	selectTypeFields map[string]string
	allProxyNames    map[string]*ast.TypeSpec

	typeInfo *types.Info

	newIdents map[string]any

	// Points to the AssignStmt while its children are being traversed
	assignCursor *astutil.Cursor
	// True when one of the AssignStmt children was replaced by a setter call
	addedVars bool
}

func newMethodProxifier(
	method *ast.FuncDecl,
	fields []*Field,
	typeInfo *types.Info,
	allProxyNames map[string]*ast.TypeSpec,
) *methodProxifier {
	selectTypeFields := make(map[string]string)
	for _, f := range fields {
		if f.selectTypeName != "" {
			selectTypeFields[f.fieldName] = f.selectTypeName
		}
	}

	return &methodProxifier{
		method:           method,
		selectTypeFields: selectTypeFields,
		allProxyNames:    allProxyNames,
		typeInfo:         typeInfo,
		newIdents:        make(map[string]any),
	}
}

func (p *methodProxifier) proxify() {
	astutil.Apply(p.method.Body, replaceReassignment, nil)
	astutil.Apply(p.method.Body, p.down, p.up)
}

func (p *methodProxifier) down(c *astutil.Cursor) bool {
	assign, ok := c.Node().(*ast.AssignStmt)
	if !ok {
		return true
	}

	p.assignCursor = c

	// Recursive apply so the p.assignCursor doesn't move
	astutil.Apply(assign, nil, p.up)

	p.assignCursor = nil
	if p.addedVars {
		assign.Tok = token.DEFINE
	}
	p.addedVars = false

	// End this branch because the recursive one already covered the rest
	// Also afaik nested assign statements are impossible in go (something like x = (y = 5))
	return false
}

func (p *methodProxifier) up(c *astutil.Cursor) bool {
	selector, ok := c.Node().(*ast.SelectorExpr)
	if !ok {
		return true
	}
	p.proxifySelector(selector, c)
	return true
}

func (p *methodProxifier) proxifySelector(
	selector *ast.SelectorExpr,
	c *astutil.Cursor,
) {
	expr := p.replaceNestedSelector(selector)

	_, ok := c.Parent().(*ast.AssignStmt)
	isLhsAssign := ok && c.Name() == "Lhs"

	if isLhsAssign {
		lhsReplacement, setterCall := p.setterOr(selector, expr)
		c.Replace(lhsReplacement)
		if setterCall != nil {
			p.assignCursor.InsertAfter(setterCall)
		}
		return
	}
	_, ok = c.Parent().(*ast.SelectorExpr)
	if !ok {
		getterCall := p.getterOr(selector, expr)
		if getterCall != nil {
			c.Replace(getterCall)
		}
	}
}

// Checks if a SelectorExpr has a nested SelectorExpr and replaces
// it with a getter call if necessary.
// In any case the original selector expression is returned.
// It is needed for being type checked while the replacement
// getter call is not.
func (p *methodProxifier) replaceNestedSelector(selector *ast.SelectorExpr) ast.Expr {
	nestedSelector, ok := selector.X.(*ast.SelectorExpr)
	if !ok {
		return selector.X
	}

	getterCall := p.getterOr(nestedSelector, nestedSelector.X)
	if getterCall != nil {
		selector.X = getterCall
	}
	return nestedSelector
}

// Checks if the selector and selectee accesses a proxy field and
// if so converts the selector into a getter call and returns it.
// Otherwise returns the unchanged selector.
func (p *methodProxifier) getterOr(selector *ast.SelectorExpr, selectee ast.Expr) ast.Expr {
	if !p.selectsProxyField(selector, selectee) {
		return selector
	}

	call := p.createGetterCall(selector)
	return call
}

// Checks if the selector and selectee access a proxy field and
// if so converts the selector into a temporary variable
// and as the second return value gives a setter call with the
// temporary variable as its argument.
// If the check fails, the original selector and nil are returned.
func (p *methodProxifier) setterOr(selector *ast.SelectorExpr, selectee ast.Expr) (ast.Expr, ast.Stmt) {
	if !p.selectsProxyField(selector, selectee) {
		return selector, nil
	}

	tempVarName := p.findUnusedIdent(selector.Sel.Name)
	tempIdent := ast.NewIdent(tempVarName)

	setterCall := p.createSetterCall(selector, tempIdent)
	setterStmt := &ast.ExprStmt{X: setterCall}

	p.addedVars = true

	return tempIdent, setterStmt
}

func (p *methodProxifier) createGetterCall(expr *ast.SelectorExpr) *ast.CallExpr {
	fieldName := expr.Sel.Name
	getterName := strcase.ToCamel(fieldName)
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   expr.X,
			Sel: ast.NewIdent(getterName),
		},
	}
	return call
}

func (p *methodProxifier) createSetterCall(expr *ast.SelectorExpr, assigned ast.Expr) *ast.CallExpr {
	fieldName := expr.Sel.Name
	setterName := fmt.Sprintf("Set%v", strcase.ToCamel(fieldName))

	selectTypeName, ok := p.selectTypeFields[fieldName]
	if ok {
		// Add a cast to the select type
		assigned = &ast.CallExpr{
			Fun:  ast.NewIdent(selectTypeName),
			Args: []ast.Expr{assigned},
		}
	}

	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   expr.X,
			Sel: ast.NewIdent(setterName),
		},
		Args: []ast.Expr{assigned},
	}
	return call
}

// Returns true when the selector is selecting at a struct field
// and the selectee's type is that of a proxy struct.
func (p *methodProxifier) selectsProxyField(selector *ast.SelectorExpr, selectee ast.Expr) bool {
	selection, ok := p.typeInfo.Selections[selector]
	if !ok || selection.Kind() != types.FieldVal {
		return false
	}

	exprType := p.typeInfo.Types[selectee]
	typeName := unwrapTypeName(exprType.Type)
	_, isProxy := p.allProxyNames[typeName]
	if !isProxy {
		return false
	}

	return true
}

func (p *methodProxifier) findUnusedIdent(ident string) string {
	ident = strcase.ToLowerCamel(ident)
	_, used := p.newIdents[ident]
	if used {
		orig := ident
		for i := 2; ; i += 1 {
			ident = fmt.Sprintf("%v%v", orig, i)
			_, used := p.newIdents[ident]
			if !used {
				break
			}
		}
	}
	p.newIdents[ident] = struct{}{}
	return ident
}

// Replaces reassignment operators with the written out version
// Example: x += 1  becomes  x = x + 1
func replaceReassignment(c *astutil.Cursor) bool {
	assign, ok := c.Node().(*ast.AssignStmt)
	if !ok {
		return true
	}

	operators := map[token.Token]token.Token{
		token.ADD_ASSIGN:     token.ADD,     // +=
		token.SUB_ASSIGN:     token.SUB,     // -=
		token.MUL_ASSIGN:     token.MUL,     // *=
		token.QUO_ASSIGN:     token.QUO,     // /=
		token.REM_ASSIGN:     token.REM,     // %=
		token.AND_ASSIGN:     token.AND,     // &=
		token.OR_ASSIGN:      token.OR,      // |=
		token.XOR_ASSIGN:     token.XOR,     // ^=
		token.SHL_ASSIGN:     token.SHL,     // <<=
		token.SHR_ASSIGN:     token.SHR,     // >>=
		token.AND_NOT_ASSIGN: token.AND_NOT, // &^=
	}

	operator, ok := operators[assign.Tok]
	if !ok {
		return true
	}

	assign.Tok = token.ASSIGN
	binary := &ast.BinaryExpr{
		X:  assign.Lhs[0],
		Op: operator,
		Y:  assign.Rhs[0],
	}
	assign.Rhs[0] = binary

	return true
}

func unwrapTypeName(typ types.Type) string {
Loop:
	for {
		switch t := typ.(type) {
		case *types.Slice:
			typ = t.Elem()
		case *types.Pointer:
			typ = t.Elem()
		default:
			break Loop
		}
	}

	namedType, ok := typ.(*types.Named)
	if ok {
		return namedType.Obj().Name()
	}
	return ""
}
