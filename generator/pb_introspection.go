package generator

// Even though the method set of pocketbase's core structs
// is unlikely to change, this file contains functions to
// extract the relevant function names directly from the
// source files to keep the hardcoded assumptions about
// the function signatures at a minimum and hopefully
// reduce maintenance.

import (
	"go/ast"
	"go/types"
	"log"
	"maps"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/packages"
)

var pbInfo *pocketBaseInfo

type pocketBaseInfo struct {
	pkg *packages.Package

	// Return type string -> method name
	recordGetters map[string]string

	// All exported names of *core.Record
	allRecordNames map[string]any

	baseProxyType *types.Named
}

func newPocketBaseInfo() *pocketBaseInfo {
	info := &pocketBaseInfo{
		pkg: loadPbCorePackage(),
	}
	info.collectRecordGetters()
	info.collectRecordNames()
	info.collectBaseProxyType()

	return info
}

func (p *pocketBaseInfo) shadowsRecord(proxyStruct *types.Named) (bool, []string) {
	proxyNames := extractNamesWithEmbedded(proxyStruct, p.baseProxyType)
	shadowed := make([]string, 0)

	for name := range proxyNames {
		if _, ok := p.allRecordNames[name]; ok {
			shadowed = append(shadowed, name)
		}
	}

	return len(shadowed) > 0, shadowed
}

func (p *pocketBaseInfo) collectRecordGetters() {
	pkg := p.pkg
	recordSrcPath := filepath.Join(pkg.Dir, "record_model.go")

	i := slices.Index(pkg.CompiledGoFiles, recordSrcPath)
	if i == -1 {
		log.Fatal("Could not find record_model.go")
	}

	f := pkg.Syntax[i]
	p.recordGetters = make(map[string]string)

	ast.Inspect(f, func(n ast.Node) bool {
		returnType := getterReturnType(n)
		if returnType == nil {
			return true
		}

		typeName := nodeString(returnType)
		funcName := n.(*ast.FuncDecl).Name.Name
		p.recordGetters[typeName] = funcName

		return false
	})
}

func (p *pocketBaseInfo) collectRecordNames() {
	recordObj := p.pkg.Types.Scope().Lookup("Record")
	recordNamedType := recordObj.Type().(*types.Named)
	p.allRecordNames = extractNamesWithEmbedded(recordNamedType, nil)
}

func (p *pocketBaseInfo) collectBaseProxyType() {
	baseProxyObj := p.pkg.Types.Scope().Lookup("BaseRecordProxy")
	p.baseProxyType = baseProxyObj.Type().(*types.Named)
}

func extractNamesWithEmbedded(namedStructType *types.Named, ignoreType *types.Named) map[string]any {
	allNames := make(map[string]any)
	queue := []*types.Named{namedStructType}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		names, embedded := extractNames(current)
		for _, e := range embedded {
			if ignoreType == nil || e.Obj().Name() != ignoreType.Obj().Name() {
				queue = append(queue, e)
			}
		}
		maps.Copy(allNames, names)
	}
	return allNames
}

// Returns the exported names (fields and methods) of the struct as a map
// and the exported embedded types as a list.
func extractNames(namedStructType *types.Named) (map[string]any, []*types.Named) {
	names := make(map[string]any)
	embedded := make([]*types.Named, 0)

	structType, ok := namedStructType.Underlying().(*types.Struct)
	if !ok {
		return names, embedded
	}

	pointerType := types.NewPointer(namedStructType)
	methodSet := types.NewMethodSet(pointerType)
	_ = methodSet

	for i := range methodSet.Len() {
		selection := methodSet.At(i)
		funcType := selection.Obj().(*types.Func)
		recv := funcType.Signature().Recv()
		pointerRecv, ok := recv.Type().(*types.Pointer)
		if !ok {
			continue
		}
		recvType := pointerRecv.Elem()
		if funcType.Exported() && types.Identical(recvType, namedStructType) {
			funcName := funcType.Name()
			names[funcName] = struct{}{}
		}
	}

	for i := range structType.NumFields() {
		field := structType.Field(i)
		if field.Exported() && !field.Anonymous() {
			names[field.Name()] = struct{}{}
		} else if field.Exported() && field.Anonymous() {
			named, ok := unwrapPointer(field.Type()).(*types.Named)
			if ok {
				embedded = append(embedded, named)
			}
		}
	}

	return names, embedded
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

func loadPbCorePackage() *packages.Package {
	importPath := "github.com/pocketbase/pocketbase/core"
	conf := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedCompiledGoFiles |
			packages.NeedFiles,
	}
	pkgs, err := packages.Load(conf, importPath)
	if err != nil {
		log.Fatal(err)
	}
	if len(pkgs) != 1 {
		log.Fatal("Error: Could not identify the pocketbase core package")
	}

	return pkgs[0]
}

func unwrapPointer(typ types.Type) types.Type {
	pointer, ok := typ.(*types.Pointer)
	if ok {
		return pointer.Elem()
	}
	return typ
}

type Importer struct{}

func (i *Importer) Import(path string) (*types.Package, error) {
	conf := &packages.Config{
		Mode: packages.NeedTypes,
	}
	pkgs, err := packages.Load(conf, path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		log.Fatalf("Could not identify package: %v", path)
	}
	return pkgs[0].Types, nil
}
