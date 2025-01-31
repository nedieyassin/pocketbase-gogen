package generator

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"log"
	"slices"

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
			newMethodProxifier(m, fields, info, parser).proxify()
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
	parser           *Parser

	typeInfo   *types.Info
	blockStmts map[token.Pos]*ast.BlockStmt

	newIdents map[string]any

	// Points to the AssignStmt while its children are being traversed
	assignCursor *astutil.Cursor
	// True when one of the AssignStmt children was replaced by a setter call
	addedVars  bool
	assignMove *assignMove
}

func newMethodProxifier(
	method *ast.FuncDecl,
	fields []*Field,
	typeInfo *types.Info,
	parser *Parser,
) *methodProxifier {
	selectTypeFields := make(map[string]string)
	for _, f := range fields {
		if f.selectTypeName != "" {
			selectTypeFields[f.fieldName] = f.selectTypeName
		}
	}

	p := &methodProxifier{
		method:           method,
		selectTypeFields: selectTypeFields,
		allProxyNames:    parser.structNames,
		parser:           parser,
		typeInfo:         typeInfo,
		newIdents:        make(map[string]any),
	}
	return p
}

func (p *methodProxifier) proxify() {
	astutil.Apply(p.method.Body, replaceReassignment, nil)
	astutil.Apply(p.method.Body, p.down, p.up)
}

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
	assign, ok := c.Node().(*ast.AssignStmt)
	if !ok {
		return true
	}

	p.assignCursor = c
	p.assignMove = nil

	// Recursive apply so the p.assignCursor doesn't move
	// Also traverse left first so the getters are already
	// changed when the setters are taking their arg
	p.traverseAssign(assign, p.traverseRight)
	p.traverseAssign(assign, p.traverseLeft)

	p.applyAssignMove()
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
	switch n := c.Node().(type) {
	case *ast.SelectorExpr:
		p.proxifySelector(n, c)
	case *ast.RangeStmt:
		p.proxifyRangeStmt(n)
	}
	return true
}

type assignMove struct {
	assign      *ast.AssignStmt
	targetBock  *ast.BlockStmt
	targetIndex int
	toMove      []ast.Expr
}

func (p *methodProxifier) initAssignMove(targetBlock *ast.BlockStmt, targetIndex int) {
	if p.assignMove != nil {
		return
	}
	p.assignMove = &assignMove{
		assign:      p.assignCursor.Node().(*ast.AssignStmt),
		targetBock:  targetBlock,
		targetIndex: targetIndex,
		toMove:      make([]ast.Expr, 0),
	}
}

func (p *methodProxifier) applyAssignMove() {
	if p.assignMove == nil {
		return
	}
	if len(p.assignMove.toMove) == 0 {
		return
	}
	assign := p.assignMove.assign

	moved := &ast.AssignStmt{
		Lhs: make([]ast.Expr, 0),
		Tok: token.DEFINE,
		Rhs: make([]ast.Expr, 0),
	}

	for _, s := range p.assignMove.toMove {
		i := slices.Index(assign.Lhs, s)
		if i == -1 {
			panic("Lift failed")
		}
		moved.Lhs = append(moved.Lhs, s)
		moved.Rhs = append(moved.Rhs, assign.Rhs[i])
	}

	if len(moved.Lhs) == len(assign.Lhs) {
		p.assignCursor.Replace(&ast.EmptyStmt{Implicit: false})
	} else {
		assign.Lhs = slices.DeleteFunc(
			assign.Lhs,
			func(e ast.Expr) bool { return slices.Contains(moved.Lhs, e) },
		)
		assign.Rhs = slices.DeleteFunc(
			assign.Rhs,
			func(e ast.Expr) bool { return slices.Contains(moved.Rhs, e) },
		)
	}

	block := p.assignMove.targetBock
	index := p.assignMove.targetIndex
	block.List = slices.Insert(block.List, index, ast.Stmt(moved))
}

func (p *methodProxifier) proxifySelector(
	selector *ast.SelectorExpr,
	c *astutil.Cursor,
) {
	p.replaceNestedSelector(selector)

	assign, ok := c.Parent().(*ast.AssignStmt)
	isLhsAssign := ok && c.Name() == "Lhs"

	if isLhsAssign {
		replacement, setterCall := p.setterOr(selector)
		if setterCall == nil {
			p.assignCursor.Replace(replacement)
			return
		}
		c.Replace(replacement)

		block, insertIndex, lift := p.findSetterBlock(p.assignCursor.Parent(), assign)
		if block == nil || insertIndex < 0 {
			pos := p.parser.Fset.Position(assign.TokPos)
			log.Fatalf("%v: An error ocurred while trying to convert the assign statement to a proxy setter call. Try using a different syntax.", pos)
		}

		block.List = slices.Insert(block.List, insertIndex, setterCall)
		if lift {
			p.initAssignMove(block, insertIndex)
			p.assignMove.toMove = append(p.assignMove.toMove, replacement.(ast.Expr))
		}
		return
	}

	_, ok = c.Parent().(*ast.SelectorExpr)
	_, ok2 := c.Parent().(*ast.RangeStmt)
	if !ok && !ok2 {
		getterCall := p.getterOr(selector)
		if getterCall != nil {
			c.Replace(getterCall)
		}
	}
}

func (p *methodProxifier) proxifyRangeStmt(stmt *ast.RangeStmt) {
	value, ok := stmt.Value.(*ast.SelectorExpr)
	ok = ok && p.selectsProxyField(value)
	if ok {
		tempVarName := p.findUnusedIdent(value.Sel.Name)
		tempVarIdent := ast.NewIdent(tempVarName)
		setterCall := p.createSetterCall(value, tempVarIdent)
		stmt.Value = tempVarIdent

		var setterStmt ast.Stmt = &ast.ExprStmt{X: setterCall}
		stmt.Body.List = slices.Insert(stmt.Body.List, 0, setterStmt)
	}

	key, ok2 := stmt.Key.(*ast.SelectorExpr)
	ok2 = ok2 && p.selectsProxyField(key)
	if ok2 {
		tempVarName := p.findUnusedIdent(key.Sel.Name)
		tempVarIdent := ast.NewIdent(tempVarName)
		setterCall := p.createSetterCall(key, tempVarIdent)
		stmt.Key = tempVarIdent

		var setterStmt ast.Stmt = &ast.ExprStmt{X: setterCall}
		stmt.Body.List = slices.Insert(stmt.Body.List, 0, setterStmt)
	}

	if ok || ok2 {
		stmt.Tok = token.DEFINE
	}

	selector, ok := stmt.X.(*ast.SelectorExpr)
	if ok {
		getterCall := p.getterOr(selector)
		if getterCall != nil {
			stmt.X = getterCall
		}
	}
}

// Checks if a SelectorExpr has a nested SelectorExpr and replaces
// it with a getter call if necessary.
func (p *methodProxifier) replaceNestedSelector(selector *ast.SelectorExpr) {
	nestedSelector, ok := selector.X.(*ast.SelectorExpr)
	if !ok {
		return
	}

	getterCall := p.getterOr(nestedSelector)
	if getterCall != nil {
		p.typeInfo.Types[getterCall] = p.typeInfo.Types[nestedSelector]
		selector.X = getterCall
	}
}

// Checks if the selector accesses a proxy field and
// if so converts the selector into a getter call and returns it.
// Otherwise returns the unchanged selector.
func (p *methodProxifier) getterOr(selector *ast.SelectorExpr) ast.Expr {
	if !p.selectsProxyField(selector) {
		return selector
	}

	call := p.createGetterCall(selector)
	return call
}

// Checks if the selector accesses a proxy field and
// if so converts the selector into a temporary variable
// and as the second return value gives a setter call with the
// temporary variable as its argument.
// If the check fails, the original selector and nil are returned.
func (p *methodProxifier) setterOr(selector *ast.SelectorExpr) (ast.Node, ast.Stmt) {
	if !p.selectsProxyField(selector) {
		return selector, nil
	}
	var replacement ast.Node
	var setterStmt ast.Stmt
	assign, ok := p.assignCursor.Node().(*ast.AssignStmt)
	multiAssign := ok && len(assign.Lhs) > 1

	if multiAssign {
		tempVarName := p.findUnusedIdent(selector.Sel.Name)
		tempVarIdent := ast.NewIdent(tempVarName)

		setterCall := p.createSetterCall(selector, tempVarIdent)

		replacement = tempVarIdent
		setterStmt = &ast.ExprStmt{X: setterCall}
		p.addedVars = true
	} else {
		setterArg := assign.Rhs[0]
		setterCall := p.createSetterCall(selector, setterArg)
		replacement = &ast.ExprStmt{X: setterCall}
	}

	return replacement, setterStmt
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

// Returns true when the selector is selecting a struct field
// and the struct is a proxy.
func (p *methodProxifier) selectsProxyField(selector *ast.SelectorExpr) bool {
	selection, ok := p.typeInfo.Selections[selector]
	if !ok || selection.Kind() != types.FieldVal {
		return false
	}

	exprType := p.typeInfo.Types[selector.X]
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

// Finds the block that a setter needs to be inserted to as the
// replacement for a proxy field assignment.
// Returns the block, the insertion index and a boolean for whether
// the assign has to move out of its parent because setter syntax does
// not work inside init and post statements of if/for statements.
func (p *methodProxifier) findSetterBlock(assignParent ast.Node, assign ast.Stmt) (*ast.BlockStmt, int, bool) {
	switch n := assignParent.(type) {
	case *ast.BlockStmt:
		return n, slices.Index(n.List, assign), false

	case *ast.ForStmt:
		if assign == n.Init {
			parentContaining, i := p.parentSetterBlock(n)
			return parentContaining, i, true
		} else if assign == n.Post {
			return n.Body, len(n.Body.List), true
		}

	case *ast.IfStmt:
		if assign == n.Init {
			parentContaining, i := p.parentSetterBlock(n)
			return parentContaining, i, true
		}

	case *ast.SwitchStmt:
		if assign == n.Init {
			parentContaining, i := p.parentSetterBlock(n)
			return parentContaining, i, true
		}

	case *ast.TypeSwitchStmt:
		if assign == n.Init {
			parentContaining, i := p.parentSetterBlock(n)
			return parentContaining, i, true
		}

	}
	return nil, -1, false
}

func (p *methodProxifier) parentSetterBlock(parent ast.Stmt) (*ast.BlockStmt, int) {
	containingBlock := p.findContainingBlock(parent)
	index := slices.Index(containingBlock.List, parent)
	return containingBlock, index
}

func (p *methodProxifier) findContainingBlock(node ast.Node) *ast.BlockStmt {
	var containingBlock *ast.BlockStmt

	finder := func(n ast.Node) bool {
		if node == n {
			return false
		}
		block, ok := n.(*ast.BlockStmt)
		if ok {
			containingBlock = block
		}
		return true
	}
	ast.Inspect(p.method.Body, finder)

	return containingBlock
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
