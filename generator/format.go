package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"strings"

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/imports"
)

func printAST(f *ast.File, fset *token.FileSet, filename string) []byte {
	buf := &bytes.Buffer{}
	if err := format.Node(buf, fset, f); err != nil {
		log.Fatal(err)
	}

	sourceCode, err := imports.Process(filename, buf.Bytes(), nil)
	if err != nil {
		log.Fatal(err)
	}

	return sourceCode
}

// Converts an ast node to a string
func nodeString(n ast.Node) string {
	fset := token.NewFileSet()
	sb := &strings.Builder{}
	err := format.Node(sb, fset, n)
	if err != nil {
		log.Fatal(err)
	}
	return sb.String()
}

func toIdentifier(s string) *ast.Ident {
	validated := validateIdentifier(s)
	return ast.NewIdent(validated)
}

// Validates if the given string s can be used
// as an identifier in go source code.
// If not, it appends a '_' and checks again.
// Errors if the string is still not valid.
func validateIdentifier(s string) string {
	origS := s
	parsed, err := parser.ParseExpr(s)
	_, ok := parsed.(*ast.Ident)
	// This check fails e.g. when the field name is a reserved go keyword
	if err != nil || !ok {
		s += "_" // Add a _ to circumvent the reservation
		parsed, err = parser.ParseExpr(s)
		_, ok = parsed.(*ast.Ident)
	}
	if err != nil || !ok {
		log.Fatalf("Error: Encoutered `%v`, which can not be used as a go identifier", origS)
	}

	return s
}

// Returns true if the given string can be used as a package name
func validatePackageName(packageName string) bool {
	packageDecl := fmt.Sprintf("package %v", packageName)
	_, err := parser.ParseFile(token.NewFileSet(), "x.go", packageDecl, parser.SkipObjectResolution)

	return err == nil
}

func getterName(varName string) string {
	getterName := strcase.ToCamel(varName)
	if getterName == "Id" {
		// Have to use GetId to not shadow the identically named core.Record.Id field
		getterName = "Get" + getterName
	}
	return getterName
}

func setterName(varName string) string {
	setterName := "Set" + strcase.ToCamel(varName)
	return setterName
}
