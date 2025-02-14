package generator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

func DebugTestPrint(t *testing.T) {
	inFile := "../example/template.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, inFile, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}

	outFile := "../ast.txt"
	writer, err := os.Create(outFile)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	ast.Fprint(writer, fset, f, nil)
}
