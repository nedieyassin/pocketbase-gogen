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
	//printAST("example/example.go")

	decls := proxiesFromGoFile("example/example.go")
	writeDecls(decls, "example_proxies.go")
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
