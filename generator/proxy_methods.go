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

	proxyMethods := make([]ast.Decl, len(methods))
	for i, m := range methods {
		proxyMethods[i] = proxifyMethod(m, fields)
	}

	return proxyMethods
}

func proxifyMethod(funcDecl *ast.FuncDecl, fields []*Field) *ast.FuncDecl {
	proxifier := newMethodProxifier(funcDecl, fields)
	proxifier.proxify()
	return funcDecl
}

type methodProxifier struct {
	method           *ast.FuncDecl
	recvName         string
	fieldNames       map[string]any
	selectTypeFields map[string]string

	newIdents map[string]any

	// Set while the children of an AssignStmt are traversed
	assignCursor *astutil.Cursor
	// True when one of the AssignStmt children was replaced by a setter call
	addedVars bool
}

func newMethodProxifier(method *ast.FuncDecl, fields []*Field) *methodProxifier {
	fieldNames := make(map[string]any, len(fields))
	selectTypeFields := make(map[string]string)
	for _, f := range fields {
		fieldNames[f.fieldName] = struct{}{}
		if f.selectTypeName != "" {
			selectTypeFields[f.fieldName] = f.selectTypeName
		}
	}

	recvName := method.Recv.List[0].Names[0].Name
	return &methodProxifier{
		method:           method,
		recvName:         recvName,
		fieldNames:       fieldNames,
		selectTypeFields: selectTypeFields,
		newIdents:        make(map[string]any),
	}
}

func (p *methodProxifier) proxify() {
	astutil.Apply(p.method.Body, p.down, p.up)
}

// Using the traverseLeft or traverseRight function as the direction
// argument, this function traverses the expressions to the
// left or right of the assign statement.
func (p *methodProxifier) traverseAssign(assign *ast.AssignStmt, direction astutil.ApplyFunc) {
	astutil.Apply(assign, direction, p.up)
}

func (p *methodProxifier) traverseLeft(c *astutil.Cursor) bool {
	_, ok := c.Parent().(*ast.AssignStmt)
	return !ok || c.Name() == "Lhs"
}

func (p *methodProxifier) traverseRight(c *astutil.Cursor) bool {
	_, ok := c.Parent().(*ast.AssignStmt)
	return !ok || c.Name() == "Rhs"
}

func (p *methodProxifier) down(c *astutil.Cursor) bool {
	n, ok := c.Node().(*ast.AssignStmt)
	if !ok {
		return true
	}

	p.assignCursor = c

	// Traverse right hand side first
	p.traverseAssign(n, p.traverseRight)
	p.traverseAssign(n, p.traverseLeft)

	if p.addedVars {
		n.Tok = token.DEFINE
	}

	p.assignCursor = nil
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
	if _, ok := c.Parent().(*ast.AssignStmt); ok && c.Name() == "Lhs" {
		// Left hand assignment children get replaced by setters
		p.replaceWithSetter(expr, c)
	} else {
		// Getter for everything else
		p.replaceWithGetter(expr, c)
	}
}

func (p *methodProxifier) replaceWithGetter(expr *ast.SelectorExpr, c *astutil.Cursor) {
	getterCall := p.createGetterCall(expr)
	c.Replace(getterCall)
	return
}

func (p *methodProxifier) replaceWithSetter(expr *ast.SelectorExpr, c *astutil.Cursor) {
	tempVarName := p.findUnusedIdent(expr.Sel.Name)
	tempIdent := ast.NewIdent(tempVarName)

	setterCall := p.createSetterCall(expr, tempIdent)
	c.Replace(tempIdent)

	p.assignCursor.InsertAfter(&ast.ExprStmt{X: setterCall})
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
