package javascript_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	fileanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/javascript"
)

func TestAnalyzerReturnsNormalizedDependenciesAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "src/App.tsx", `import "./Button"; import "./missing"; import React from "react";`)
	writeSource(t, root, "src/Button.tsx", "")
	index := filegraph.NewRepositoryIndex([]string{"src/App.tsx", "src/Button.tsx"})
	result, err := javascript.New().Analyze(context.Background(), fileanalyzer.FileContext{
		Root: root, Path: "src/App.tsx", Index: index,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDependencies := []fileanalyzer.Dependency{{Target: "src/Button.tsx", Kind: "import", Confidence: "exact"}}
	if !reflect.DeepEqual(result.Dependencies, wantDependencies) {
		t.Fatalf("dependencies: got %+v, want %+v", result.Dependencies, wantDependencies)
	}
	wantDiagnostics := []fileanalyzer.Diagnostic{{Reference: "./missing", Message: "unresolved local import"}}
	if !reflect.DeepEqual(result.Diagnostics, wantDiagnostics) {
		t.Fatalf("diagnostics: got %+v, want %+v", result.Diagnostics, wantDiagnostics)
	}
}

func writeSource(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
