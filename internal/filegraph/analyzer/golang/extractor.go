package golang

import (
	"go/parser"
	"go/token"
	"strconv"
)

// ExtractImports parses Go import declarations without type checking,
// compiling, or executing repository code.
func ExtractImports(filename string, source []byte) ([]string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, imported := range parsed.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return nil, err
		}
		imports = append(imports, value)
	}
	return imports, nil
}
