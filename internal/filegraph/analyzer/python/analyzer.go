// Package python analyzes Python file dependencies.
package python

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
)

// Analyzer implements the language-neutral analyzer contract for Python.
type Analyzer struct{}

// New constructs a Python analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Supports reports whether path is a Python source file.
func (*Analyzer) Supports(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".py")
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
	for _, imported := range imports {
		resolved, local := Resolve(file.Index, file.Path, imported)
		if len(resolved) == 0 {
			if local {
				result.Diagnostics = append(result.Diagnostics, analyzer.Diagnostic{
					Reference: imported.String(),
					Message:   "unresolved local import",
				})
			}
			continue
		}
		for _, target := range resolved {
			result.Dependencies = append(result.Dependencies, analyzer.Dependency{
				Target:     target,
				Kind:       "import",
				Confidence: "exact",
			})
		}
	}
	return result, nil
}
