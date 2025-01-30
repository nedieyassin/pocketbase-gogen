package generator

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/go/ast/astutil"
)

func createProxyMethods(methods []*ast.FuncDecl, fields []*Field) []ast.Decl {
	if len(methods) == 0 {
		return nil
	}

	fieldNames := make(map[string]any, len(fields))
	for _, f := range fields {
		fieldNames[f.fieldName] = struct{}{}
	}

	proxyMethods := make([]ast.Decl, len(methods))
	for i, m := range methods {
		proxyMethods[i] = proxifyMethod(m, fieldNames)
	}

	return proxyMethods
}

func proxifyMethod(funcDecl *ast.FuncDecl, fieldNames map[string]any) *ast.FuncDecl {
	proxifier := newMethodProxifier(funcDecl, fieldNames)
	proxifier.proxify()
	return funcDecl
}

type methodProxifier struct {
	method     *ast.FuncDecl
	recvName   string
	fieldNames map[string]any

	newIdents map[string]any

	// Set while the children of an AssignStmt are traversed
	curAssign       *ast.AssignStmt
	curAssignCursor *astutil.Cursor
	// Current AssignStmt's child index
	curIdx int
	// True when one of the AssignStmt children was replaced by a setter call
	addedVars bool
}

func newMethodProxifier(method *ast.FuncDecl, fieldNames map[string]any) *methodProxifier {
	recvName := method.Recv.List[0].Names[0].Name
	return &methodProxifier{
		method:     method,
		recvName:   recvName,
		fieldNames: fieldNames,
		newIdents:  make(map[string]any),
		curIdx:     -1,
	}
}

func (p *methodProxifier) proxify() {
	p.traverse(p.method.Body)
}

func (p *methodProxifier) traverse(node ast.Node) {
	astutil.Apply(node, p.down, p.up)
}

func (p *methodProxifier) traverseAssign(children []ast.Expr) {
	for i, n := range children {
		p.curIdx = i
		p.traverse(n)
	}
	p.curIdx = -1
}

func (p *methodProxifier) down(c *astutil.Cursor) bool {
	n, ok := c.Node().(*ast.AssignStmt)
	if !ok {
		return true
	}

	p.curAssign = n
	p.curAssignCursor = c

	// Traverse right hand side first
	p.traverseAssign(n.Rhs)
	p.traverseAssign(n.Lhs)

	if p.addedVars {
		n.Tok = token.DEFINE
	}

	p.curAssign = nil
	p.curAssignCursor = nil
	p.addedVars = false

	return false
}

func (p *methodProxifier) up(c *astutil.Cursor) bool {
	fieldExpr, ok := p.fieldExpr(c.Node())
	if !ok {
		return true
	}
	p.replaceFieldExpr(fieldExpr, c)
	return true
}

func (p *methodProxifier) replaceFieldExpr(expr *ast.SelectorExpr, c *astutil.Cursor) {
	if p.curAssign != nil && isAssignmentChild(expr, p.curAssign.Lhs) {
		// Left hand assignment children get replaced by setters
		p.replaceWithSetter(expr)
	} else {
		// Getter for everything else
		p.replaceWithGetter(expr, c)
	}
}

func (p *methodProxifier) replaceWithGetter(expr *ast.SelectorExpr, c *astutil.Cursor) {
	getterCall := p.createGetterCall(expr)
	if p.curAssign != nil && isAssignmentChild(expr, p.curAssign.Rhs) {
		p.curAssign.Rhs[p.curIdx] = getterCall
	} else {
		c.Replace(getterCall)
	}
}

func (p *methodProxifier) replaceWithSetter(expr *ast.SelectorExpr) {
	tempVarName := p.findUnusedIdent(expr.Sel.Name)
	tempIdent := ast.NewIdent(tempVarName)

	setterCall := p.createSetterCall(expr, tempIdent)

	p.curAssign.Lhs[p.curIdx] = tempIdent
	p.curAssignCursor.InsertAfter(&ast.ExprStmt{X: setterCall})
	p.addedVars = true
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

func (p *methodProxifier) createGetterCall(expr *ast.SelectorExpr) *ast.CallExpr {
	fieldName := expr.Sel.Name
	getterName := strcase.ToCamel(fieldName)
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(p.recvName),
			Sel: ast.NewIdent(getterName),
		},
	}
	return call
}

func (p *methodProxifier) createSetterCall(expr *ast.SelectorExpr, assigned ast.Expr) *ast.CallExpr {
	fieldName := expr.Sel.Name
	setterName := fmt.Sprintf("Set%v", strcase.ToCamel(fieldName))
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(p.recvName),
			Sel: ast.NewIdent(setterName),
		},
		Args: []ast.Expr{assigned},
	}
	return call
}

// Returns a SelectorExpr, true if the given expression
// resolves to one of the proxy fields.
// Otherwise nil, false
func (p *methodProxifier) fieldExpr(node ast.Node) (*ast.SelectorExpr, bool) {
	selector, ok := node.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	exprIdent, ok := selector.X.(*ast.Ident)
	if !ok || exprIdent.Name != p.recvName {
		return nil, false
	}
	selectedName := selector.Sel.Name
	_, isFieldName := p.fieldNames[selectedName]
	if !isFieldName {
		return nil, false
	}

	return selector, true
}

func isAssignmentChild(expr *ast.SelectorExpr, assignments []ast.Expr) bool {
	for _, e := range assignments {
		selectorExpr, ok := e.(*ast.SelectorExpr)
		if ok && selectorExpr == expr {
			return true
		}
	}
	return false
}
