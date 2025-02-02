package generator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/go/ast/astutil"
)

func createProxyMethods(parser *Parser) (map[string][]ast.Decl, error) {
	conf := types.Config{Importer: &Importer{}}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	_, err := conf.Check("template", parser.Fset, []*ast.File{parser.fAst}, info)
	if err != nil {
		return nil, err
	}

	decls := make(map[string][]ast.Decl)
	for _, s := range parser.structSpecs {
		structName := s.Name.Name
		methods := parser.structMethods[structName]

		// struct name -> field name -> *Field
		allProxyFields := make(map[string]map[string]*Field)
		for structName, fields := range parser.structFields {
			allProxyFields[structName] = make(map[string]*Field)
			for _, f := range fields {
				allProxyFields[structName][f.fieldName] = f
			}
		}

		proxyMethods := make([]ast.Decl, len(methods))
		for i, m := range methods {
			proxifier := newMethodProxifier(m, info, allProxyFields, parser)
			if err := proxifier.proxify(); err != nil {
				return nil, err
			}
			proxyMethods[i] = m
		}

		decls[structName] = proxyMethods
	}

	return decls, nil
}

type methodProxifier struct {
	method         *ast.FuncDecl
	allProxyNames  map[string]*ast.TypeSpec
	allProxyFields map[string]map[string]*Field
	parser         *Parser

	typeInfo *types.Info

	newIdents map[string]any

	// Points to the AssignStmt while its children are being traversed
	assignCursor *astutil.Cursor
	// True when one of the AssignStmt children was replaced by a setter call
	addedVars  bool
	assignMove *assignMove

	err error
}

func newMethodProxifier(
	method *ast.FuncDecl,
	typeInfo *types.Info,
	allProxyFields map[string]map[string]*Field,
	parser *Parser,
) *methodProxifier {
	p := &methodProxifier{
		method:         method,
		allProxyNames:  parser.structNames,
		allProxyFields: allProxyFields,
		parser:         parser,
		typeInfo:       typeInfo,
		newIdents:      make(map[string]any),
	}
	return p
}

func (p *methodProxifier) proxify() error {
	astutil.Apply(p.method.Body, replaceReassignment, nil)
	astutil.Apply(p.method.Body, p.down, p.up)
	return p.err
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
		if p.err != nil {
			return false
		}
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
			panic("AssignStmt move failed")
		}
		moved.Lhs = append(moved.Lhs, s)
		moved.Rhs = append(moved.Rhs, assign.Rhs[i])
	}

	if len(moved.Lhs) == len(assign.Lhs) {
		p.assignCursor.Replace(&ast.EmptyStmt{Implicit: false})
	} else {
		p.addedVars = false
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
		switch replacement.(type) {
		case ast.Stmt:
			p.assignCursor.Replace(replacement)
		case ast.Expr:
			c.Replace(replacement)
		}
		if setterCall == nil {
			return
		}

		block, insertIndex, lift := p.findSetterBlock(p.assignCursor.Parent(), assign)
		if block == nil || insertIndex < 0 {
			pos := p.parser.Fset.Position(assign.TokPos)
			errMsg := fmt.Sprintf("%v: An error ocurred while trying to convert the assign statement to a proxy setter call. Try using a different syntax.", pos)
			p.err = errors.New(errMsg)
			return
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
	selectsProxy, proxyTypeName := p.selectsProxyField(value)
	ok = ok && selectsProxy
	if ok {
		tempVarName := p.findUnusedIdent(value.Sel.Name)
		tempVarIdent := ast.NewIdent(tempVarName)
		p.typeInfo.Types[tempVarIdent] = p.typeInfo.Types[stmt.Value]

		setterCall := p.createSetterCall(value, tempVarIdent, proxyTypeName)

		stmt.Value = tempVarIdent

		var setterStmt ast.Stmt = &ast.ExprStmt{X: setterCall}
		stmt.Body.List = slices.Insert(stmt.Body.List, 0, setterStmt)
	}

	key, ok2 := stmt.Key.(*ast.SelectorExpr)
	selectsProxy, proxyTypeName = p.selectsProxyField(key)
	ok2 = ok2 && selectsProxy
	if ok2 {
		tempVarName := p.findUnusedIdent(key.Sel.Name)
		tempVarIdent := ast.NewIdent(tempVarName)
		p.typeInfo.Types[tempVarIdent] = p.typeInfo.Types[stmt.Key]

		setterCall := p.createSetterCall(key, tempVarIdent, proxyTypeName)

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
	if ok, _ := p.selectsProxyField(selector); !ok {
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
	selectsProxy, proxyTypeName := p.selectsProxyField(selector)
	if !selectsProxy {
		return selector, nil
	}

	var replacement ast.Node
	var setterStmt ast.Stmt
	assign, ok := p.assignCursor.Node().(*ast.AssignStmt)
	multiAssign := ok && len(assign.Lhs) > 1

	if multiAssign {
		tempVarName := p.findUnusedIdent(selector.Sel.Name)
		tempVarIdent := ast.NewIdent(tempVarName)
		p.typeInfo.Types[tempVarIdent] = p.typeInfo.Types[selector]

		setterCall := p.createSetterCall(selector, tempVarIdent, proxyTypeName)

		replacement = tempVarIdent
		setterStmt = &ast.ExprStmt{X: setterCall}
		p.addedVars = true
	} else {
		setterArg := assign.Rhs[0]
		setterCall := p.createSetterCall(selector, setterArg, proxyTypeName)
		replacement = &ast.ExprStmt{X: setterCall}
	}

	return replacement, setterStmt
}

func (p *methodProxifier) createGetterCall(expr *ast.SelectorExpr) *ast.CallExpr {
	getterName := getterName(expr.Sel.Name)
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   expr.X,
			Sel: ast.NewIdent(getterName),
		},
	}
	return call
}

func (p *methodProxifier) createSetterCall(expr *ast.SelectorExpr, assigned ast.Expr, proxyTypeName string) *ast.CallExpr {
	fieldName := expr.Sel.Name
	setterName := setterName(fieldName)

	var isSelectType bool
	proxyField, ok := p.allProxyFields[proxyTypeName][fieldName]
	if ok {
		isSelectType = proxyField.selectTypeName != ""
	}
	if isSelectType {
		// Add a cast to the select type
		assigned = p.selectCast(assigned, proxyField)
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

func (p *methodProxifier) selectCast(assigned ast.Expr, selectTypeField *Field) ast.Expr {
	typeToAssign := p.typeInfo.Types[assigned].Type
	_, isMultiSelect := typeToAssign.(*types.Slice)

	var cast ast.Expr
	if isMultiSelect {
		// The code of the ast that is created here looks like
		// *(*[]SelectType)(unsafe.Pointer(&assigned))
		// it converts the assigned []int to []SelectType without copying.
		// It is a valid use of unsafe.Pointer because SelectType is guaranteed to
		// have int as its underlying type
		slicePtr := &ast.UnaryExpr{X: assigned, Op: token.AND}
		unsafePtrFunc := &ast.SelectorExpr{
			X:   ast.NewIdent("unsafe"),
			Sel: ast.NewIdent("Pointer"),
		}
		unsafePtr := &ast.CallExpr{
			Fun:  unsafePtrFunc,
			Args: []ast.Expr{slicePtr},
		}
		sliceType := &ast.ArrayType{Elt: ast.NewIdent(selectTypeField.selectTypeName)}
		slicePtrType := &ast.StarExpr{X: sliceType}
		slicePtrTypeCast := &ast.ParenExpr{X: slicePtrType}
		pointerCast := &ast.CallExpr{Fun: slicePtrTypeCast, Args: []ast.Expr{unsafePtr}}
		cast = &ast.StarExpr{X: pointerCast}
	} else {
		cast = &ast.CallExpr{
			Fun:  ast.NewIdent(selectTypeField.selectTypeName),
			Args: []ast.Expr{assigned},
		}
	}

	return cast
}

// Returns true when the selector is selecting a struct field
// and the struct is a proxy.
// When true also returns the name of the proxy struct.
func (p *methodProxifier) selectsProxyField(selector *ast.SelectorExpr) (bool, string) {
	selection, ok := p.typeInfo.Selections[selector]
	if !ok || selection.Kind() != types.FieldVal {
		return false, ""
	}

	exprType := p.typeInfo.Types[selector.X]
	typeName := unwrapTypeName(exprType.Type)
	_, isProxy := p.allProxyNames[typeName]
	if !isProxy {
		return false, ""
	}

	if selector.Sel.Name == "Id" {
		// Id is the only public field of core.Record
		// and thus not a proxy field
		return false, ""
	}

	return true, typeName
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
		return n, slices.Index(n.List, assign) + 1, false

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
	finder := &containerFinder{node: node}
	astutil.Apply(p.method, finder.traverse, nil)

	return finder.containingBlock
}

type containerFinder struct {
	containingBlock, currentBlock *ast.BlockStmt
	node                          ast.Node
}

func (f *containerFinder) traverse(c *astutil.Cursor) bool {
	if c.Name() == "Node" {
		// skip root
		return true
	}
	if f.node == c.Node() {
		f.containingBlock = f.currentBlock
		return false
	}
	block, ok := c.Node().(*ast.BlockStmt)
	if ok {
		prevBlock := f.currentBlock
		f.currentBlock = block
		astutil.Apply(block, f.traverse, nil)
		f.currentBlock = prevBlock
		return false
	}
	return true
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
