package generator

import (
	"go/ast"

	"github.com/snonky/astpos/astpos"
)

func GenerateUtils(templateParser *Parser, savePath, packageName string) ([]byte, error) {
	decls := utilsFromTemplate(templateParser)

	f := wrapGeneratedDeclarations(decls, packageName)

	f, fset := astpos.RewritePositions(f)
	sourceCode, err := printAST(f, fset, savePath)
	if err != nil {
		return nil, err
	}

	return sourceCode, nil
}

func utilsFromTemplate(parser *Parser) []ast.Decl {
	structNames := make([]string, len(parser.structSpecs))
	for i, s := range parser.structSpecs {
		structNames[i] = s.Name.Name
	}

	decls := []ast.Decl{
		newProxyTypeConstraint(structNames),
		proxyPInterfaceTemplate,
		collectionNameUtilTemplate,
		newProxyUtilTemplate,
		wrapRecordUtilTemplate,
		relationFieldStructTemplate,
		createRelationMapDecl(structNames, parser),
	}

	return decls
}

func createRelationMapDecl(structNames []string, parser *Parser) ast.Decl {
	collectionNames := make([]string, 0, len(structNames))
	for _, structName := range structNames {
		collectionName := parser.collectionNames[structName]
		if collectionName != "" {
			collectionNames = append(collectionNames, collectionName)
		}
	}

	relationMap := createRelationMap(structNames, parser)
	mapDecl := newRelationMapDecl(collectionNames, relationMap)
	return mapDecl
}

type relationField struct {
	fieldName string
	isMulti   bool
}

func createRelationMap(structNames []string, parser *Parser) map[string]map[string][]relationField {
	allRelations := make(map[string]map[string][]relationField)
	for _, structName := range structNames {
		collectionName := parser.collectionNames[structName]
		if collectionName == "" {
			continue
		}

		collectionRelations := relationsOfCollection(structName, parser)
		if len(collectionRelations) > 0 {
			allRelations[collectionName] = collectionRelations
		}
	}

	return allRelations
}

// Returns a map of
// collection name -> fields of that collection's proxy type in the struct
func relationsOfCollection(structName string, parser *Parser) map[string][]relationField {
	relations := make(map[string][]relationField)
	for _, f := range parser.structFields[structName] {
		relatedTypeName, isMulti := relatedTypeAndMulti(f, parser)
		if relatedTypeName == "" {
			continue
		}

		collectionName := parser.collectionNames[relatedTypeName]
		if collectionName == "" {
			continue
		}

		rField := relationField{fieldName: f.schemaName, isMulti: isMulti}

		rels, ok := relations[collectionName]
		if !ok {
			rels = make([]relationField, 0)
		}
		relations[collectionName] = append(rels, rField)
	}
	return relations
}

func relatedTypeAndMulti(field *Field, parser *Parser) (string, bool) {
	fieldTypeName := baseType(field.fieldType).Name

	_, ok := parser.structNames[fieldTypeName]
	if !ok {
		// Is not a relation type
		return "", false
	}

	multiplicity := relationType(field.fieldType)
	isMulti := multiplicity == multiRel

	return fieldTypeName, isMulti
}
