package main

// Even though the method set of pocketbase's core structs
// is unlikely to change, this file contains functions to
// extract the relevant function names directly from the
// source files to keep the hardcoded assumptions about
// the function signatures at a minimum and hopefully
// reduce maintenance.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// Introspects the pocketbase core.Record code and
// returns the available getters as a map of
// [return type string] -> [method name].
// E.g. "bool" -> "GetBool"
func recordGetters() map[string]string {
	filepath := findRecordSourceCodePath()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath, nil, parser.SkipObjectResolution)
	if err != nil {
		log.Fatal(err)
	}

	getters := collectGetters(f)

	nameMap := make(map[string]string, len(getters))
	for k, v := range getters {
		nameMap[nodeString(k)] = v
	}

	return nameMap
}

func collectGetters(root ast.Node) map[ast.Expr]string {
	getters := make(map[ast.Expr]string)

	ast.Inspect(root, func(n ast.Node) bool {
		returnType := getterReturnType(n)
		if returnType == nil {
			return true
		}

		funcName := n.(*ast.FuncDecl).Name.Name
		getters[returnType] = funcName

		return false
	})

	return getters
}

// Checks if the node n is a specific getter method
// of the core.Record struct and returns
// its return type expression.
// Returns nil if the node is not a getter.
func getterReturnType(n ast.Node) ast.Expr {
	funcDecl, ok := n.(*ast.FuncDecl)
	if !ok {
		return nil
	}

	funcName := funcDecl.Name.Name
	if len(funcName) < 4 || funcName[:3] != "Get" {
		return nil
	}

	recv := funcDecl.Recv
	if len(recv.List) != 1 {
		return nil
	}

	recvType, ok := recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return nil
	}

	recvIdent, ok := recvType.X.(*ast.Ident)
	if !ok || recvIdent.Name != "Record" {
		return nil
	}

	paramFields := funcDecl.Type.Params.List
	if len(paramFields) != 1 {
		return nil
	}

	paramType, ok := paramFields[0].Type.(*ast.Ident)
	if !ok || paramType.Name != "string" {
		return nil
	}

	returnFields := funcDecl.Type.Results.List
	if len(returnFields) != 1 {
		return nil
	}

	returnType, ok := returnFields[0].Type.(*ast.Ident)
	if ok && returnType.Name == "any" {
		return nil
	}
	_, ok = returnFields[0].Type.(*ast.InterfaceType)
	if ok {
		return nil
	}

	return returnFields[0].Type
}

func findRecordSourceCodePath() string {
	importPath := "github.com/pocketbase/pocketbase/core"
	packages, err := packages.Load(&packages.Config{Mode: packages.NeedFiles}, importPath)
	if err != nil {
		log.Fatal(err)
	}
	if len(packages) != 1 {
		log.Fatal("Error: Could not identify the pocketbase package directory")
	}

	recordModelFilepath := filepath.Join(packages[0].Dir, "record_model.go")

	_, err = os.Stat(recordModelFilepath)
	if err != nil {
		log.Fatal("Error: The record_model.go source code file could not be found")
	}

	return recordModelFilepath
}
