package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/imports"
)

func writeDecls(decls []ast.Decl, filename string) {
	fset := token.NewFileSet()
	//f, _ := parser.ParseFile(fset, filename, "package pack", parser.SkipObjectResolution)

	//f.Decls = decls

	f := &ast.File{
		Name:  &ast.Ident{Name: "main"},
		Decls: decls,
	}

	buf := &bytes.Buffer{}
	if err := format.Node(buf, fset, f); err != nil {
		log.Fatal(err)
	}

	sourceCode, err := imports.Process(filename, buf.Bytes(), nil)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	_, err = out.Write(sourceCode)
	if err != nil {
		log.Fatal(err)
	}
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
