// Package javascript analyzes JavaScript and TypeScript file dependencies.
package javascript

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
)

// Analyzer implements the language-neutral analyzer contract for JS and TS.
type Analyzer struct{}

// New constructs a JavaScript and TypeScript analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Supports reports whether path has a supported JavaScript or TypeScript extension.
func (*Analyzer) Supports(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".jsx", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

// Analyze extracts and resolves local imports without executing source code.
func (*Analyzer) Analyze(_ context.Context, file analyzer.FileContext) (analyzer.Result, error) {
	source, err := analyzer.ReadSource(file.Root, file.Path)
	if err != nil {
		return analyzer.Result{}, fmt.Errorf("read %q: %w", file.Path, err)
	}
	imports, err := ExtractImports(source)
	if err != nil {
		return analyzer.Result{Diagnostics: []analyzer.Diagnostic{{Message: err.Error()}}}, nil
	}

	result := analyzer.Result{}
	for _, specifier := range imports {
		resolved, ok := Resolve(file.Index, file.Path, specifier)
		if !ok {
			result.Diagnostics = append(result.Diagnostics, analyzer.Diagnostic{
				Reference: specifier,
				Message:   "unresolved local import",
			})
			continue
		}
		result.Dependencies = append(result.Dependencies, analyzer.Dependency{
			Target:     resolved,
			Kind:       "import",
			Confidence: "exact",
		})
	}
	return result, nil
}
