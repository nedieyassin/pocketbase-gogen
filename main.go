package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
)

func main() {
	loadTemplateASTs()
	loadRecordGetters()

	/*
		collections := parseSchemaJson([]byte(example.JsonData), false)
		t := newSchemaTranslator(collections)
		decls := t.translate()
		f := writeCollectionStructs(decls, "example")
		f, fset := astpos.RewritePositions(f)
		writeAST(f, fset, "example/example_prelim.go")

		decls = proxiesFromGoFile("example/example_prelim.go")
		f = writeProxyStructs(decls, "example")
		f, fset = astpos.RewritePositions(f)
		writeAST(f, fset, "example/example_proxies.go")
	*/
}

func printAST(filename string) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, filename, nil, parser.SkipObjectResolution)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("ast.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	ast.Fprint(out, fset, f, nil)
}
