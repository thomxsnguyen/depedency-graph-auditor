package golang_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	fileanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/golang"
)

func TestAnalyzerReturnsAllLocalPackageFiles(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module example.com/project\n")
	writeGoFixture(t, root, "cmd/server/main.go", `package main
import (
	"example.com/project/internal/config"
	"example.com/project/internal/missing"
	"fmt"
)
`)
	writeGoFixture(t, root, "internal/config/load.go", "package config\n")
	writeGoFixture(t, root, "internal/config/types.go", "package config\n")
	writeGoFixture(t, root, "internal/config/config_test.go", "package config_test\n")
	repository := filegraph.NewRepositoryIndex([]string{
		"cmd/server/main.go",
		"internal/config/load.go",
		"internal/config/types.go",
		"internal/config/config_test.go",
	})
	modules, _, err := golang.BuildModuleIndex(root, repository, []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := golang.New(modules).Analyze(context.Background(), fileanalyzer.FileContext{
		Root: root, Path: "cmd/server/main.go", Index: repository,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDependencies := []fileanalyzer.Dependency{
		{Target: "internal/config/load.go", Kind: "import", Confidence: "exact"},
		{Target: "internal/config/types.go", Kind: "import", Confidence: "exact"},
	}
	if !reflect.DeepEqual(result.Dependencies, wantDependencies) {
		t.Fatalf("dependencies: got %+v, want %+v", result.Dependencies, wantDependencies)
	}
	wantDiagnostics := []fileanalyzer.Diagnostic{{Reference: "example.com/project/internal/missing", Message: "unresolved local import"}}
	if !reflect.DeepEqual(result.Diagnostics, wantDiagnostics) {
		t.Fatalf("diagnostics: got %+v, want %+v", result.Diagnostics, wantDiagnostics)
	}
}

func TestAnalyzerConvertsParserFailureToDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module example.com/project\n")
	writeGoFixture(t, root, "broken.go", "package broken\nimport (\n\"fmt\"")
	repository := filegraph.NewRepositoryIndex([]string{"broken.go"})
	modules, _, err := golang.BuildModuleIndex(root, repository, []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := golang.New(modules).Analyze(context.Background(), fileanalyzer.FileContext{
		Root: root, Path: "broken.go", Index: repository,
	})
	if err != nil || len(result.Diagnostics) != 1 {
		t.Fatalf("parser result: %+v err=%v", result, err)
	}
}

func TestAnalyzerSupportsOnlyGoAndSkipsDisabledModule(t *testing.T) {
	analyzerUnderTest := golang.New(golang.ModuleIndex{})
	if !analyzerUnderTest.Supports("main.go") || analyzerUnderTest.Supports("main.py") {
		t.Fatal("unexpected extension support")
	}
	result, err := analyzerUnderTest.Analyze(context.Background(), fileanalyzer.FileContext{Path: "main.go"})
	if err != nil || len(result.Dependencies) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("disabled module result: %+v err=%v", result, err)
	}
}

func TestAnalyzerReturnsSourceReadFailure(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module example.com/project\n")
	repository := filegraph.NewRepositoryIndex([]string{"missing.go"})
	modules, _, err := golang.BuildModuleIndex(root, repository, []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := golang.New(modules).Analyze(context.Background(), fileanalyzer.FileContext{
		Root: root, Path: "missing.go", Index: repository,
	}); err == nil {
		t.Fatal("expected source read error")
	}
}

func TestAnalyzerSkipsUnsupportedNestedModuleFile(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module example.com/project\n")
	repository := filegraph.NewRepositoryIndex([]string{"nested/missing.go"})
	modules, diagnostics, err := golang.BuildModuleIndex(root, repository, []string{"go.mod", "nested/go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("nested module diagnostics: %+v", diagnostics)
	}
	result, err := golang.New(modules).Analyze(context.Background(), fileanalyzer.FileContext{
		Root: root, Path: "nested/missing.go", Index: repository,
	})
	if err != nil || len(result.Dependencies) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("nested module result: %+v err=%v", result, err)
	}
}
