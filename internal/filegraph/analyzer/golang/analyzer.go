package golang

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
)

// Analyzer implements file dependency analysis for one root Go module.
type Analyzer struct {
	modules ModuleIndex
}

// New constructs a Go analyzer backed by immutable module metadata.
func New(modules ModuleIndex) *Analyzer {
	return &Analyzer{modules: modules}
}

// Supports reports whether path is a Go source file.
func (*Analyzer) Supports(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".go")
}

// Analyze extracts and resolves repository-local Go package imports.
func (a *Analyzer) Analyze(_ context.Context, file analyzer.FileContext) (analyzer.Result, error) {
	if !a.modules.Owns(file.Path) {
		return analyzer.Result{}, nil
	}
	source, err := analyzer.ReadSource(file.Root, file.Path)
	if err != nil {
		return analyzer.Result{}, fmt.Errorf("read %q: %w", file.Path, err)
	}
	imports, err := ExtractImports(file.Path, source)
	if err != nil {
		return analyzer.Result{Diagnostics: []analyzer.Diagnostic{{Message: err.Error()}}}, nil
	}

	result := analyzer.Result{}
	for _, importPath := range imports {
		resolved := Resolve(a.modules, importPath)
		if !resolved.Local {
			continue
		}
		if len(resolved.Targets) == 0 {
			result.Diagnostics = append(result.Diagnostics, analyzer.Diagnostic{
				Reference: importPath,
				Message:   "unresolved local import",
			})
			continue
		}
		for _, target := range resolved.Targets {
			result.Dependencies = append(result.Dependencies, analyzer.Dependency{
				Target:     target,
				Kind:       "import",
				Confidence: "exact",
			})
		}
	}
	return result, nil
}
