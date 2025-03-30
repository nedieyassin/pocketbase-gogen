package generator

import "github.com/snonky/astpos/astpos"

func GenerateProxyEvents(savePath, packageName string) ([]byte, error) {
	// The event declarations are not influenced by the collection schema so just
	// save directly from template
	f := wrapGeneratedDeclarations(proxyEventCodeTemplate, packageName)

	f, fset := astpos.RewritePositions(f)
	sourceCode, err := printAST(f, fset, savePath)
	if err != nil {
		return nil, err
	}

	return sourceCode, nil
}
