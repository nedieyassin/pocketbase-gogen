package generator

import (
	"go/ast"

	"github.com/snonky/astpos/astpos"
)

func GenerateProxyHooks(templateParser *Parser, savePath, packageName string) ([]byte, error) {
	decls := hooksFromTemplate(templateParser)

	f := wrapGeneratedDeclarations(decls, packageName)

	f, fset := astpos.RewritePositions(f)
	sourceCode, err := printAST(f, fset, savePath)
	if err != nil {
		return nil, err
	}

	return sourceCode, nil
}

func hooksFromTemplate(parser *Parser) []ast.Decl {
	decls := make([]ast.Decl, 0)
	structNames := make([]string, len(parser.structSpecs))
	collectionNames := parser.collectionNames
	for i, s := range parser.structSpecs {
		structNames[i] = s.Name.Name
	}

	decls = append(decls, createEventAliases(structNames)...)
	decls = append(decls, createProxyHooksStruct(structNames))
	decls = append(decls, createProxyHooksConstructor(structNames))
	decls = append(decls, createProxyHooksRegistrationFunc(structNames, collectionNames))

	return decls
}

func createEventAliases(structNames []string) []ast.Decl {
	decls := make([]ast.Decl, 0, len(structNames)*5)
	for _, structName := range structNames {
		decls = append(decls,
			newEventTypeAliasDecl(proxyEventAliasTemplate, structName),
			newEventTypeAliasDecl(proxyEnrichEventAliasTemplate, structName),
			newEventTypeAliasDecl(proxyErrorEventAliasTemplate, structName),
			newEventTypeAliasDecl(proxyListEventAliasTemplate, structName),
			newEventTypeAliasDecl(proxyRequestEventAliasTemplate, structName),
		)
	}
	return decls
}

func createProxyHooksStruct(structNames []string) *ast.GenDecl {
	structDecl := newProxyHooksStructDecl()
	structType := structDecl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
	fieldList := make([]*ast.Field, 0, len(structNames)*19)

	templateType := proxyHooksTemplate.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
	fieldTemplates := templateType.Fields.List

	for _, structName := range structNames {
		for _, template := range fieldTemplates {
			fieldList = append(fieldList, newHookField(template, structName))
		}
	}

	structType.Fields.List = fieldList

	return structDecl
}

func createProxyHooksConstructor(structNames []string) *ast.FuncDecl {
	funcDecl := newProxyHooksConstructor()
	assignStmt := funcDecl.Body.List[0].(*ast.AssignStmt)
	structLit := assignStmt.Rhs[0].(*ast.UnaryExpr).X.(*ast.CompositeLit)
	structFieldInits := make([]ast.Expr, 0, len(structNames)*19)

	templateAssign := proxyHooksConstructorTemplate.Body.List[0].(*ast.AssignStmt)
	fieldTemplates := templateAssign.Rhs[0].(*ast.UnaryExpr).X.(*ast.CompositeLit).Elts

	for _, structName := range structNames {
		for _, template := range fieldTemplates {
			structFieldInits = append(structFieldInits, newHookConstructorArgument(template.(*ast.KeyValueExpr), structName))
		}
	}

	structLit.Elts = structFieldInits

	return funcDecl
}

func createProxyHooksRegistrationFunc(structNames []string, collectionNames map[string]string) *ast.FuncDecl {
	funcDecl := newHookRegistrationFuncDecl()
	callExprList := make([]ast.Stmt, 0, len(structNames)*19)

	callTemplates := proxyHookRegistrationTemplate.Body.List

	for _, structName := range structNames {
		collectionName := collectionNames[structName]
		for _, template := range callTemplates {
			callExprList = append(callExprList, newHookRegistrationCallExpr(template.(*ast.ExprStmt), structName, collectionName))
		}
	}

	funcDecl.Body.List = callExprList

	return funcDecl
}
