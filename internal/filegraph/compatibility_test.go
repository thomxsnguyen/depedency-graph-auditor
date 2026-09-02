package filegraph

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	fileanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	goanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/golang"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/javascript"
	pythonanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/python"
)

func TestLanguageAnalyzerMigrationPreservesCompatibilityFixture(t *testing.T) {
	root := filepath.Join("testdata", "compatibility")
	discovery, err := DiscoverRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	moduleIndex, moduleDiagnostics, err := goanalyzer.BuildModuleIndex(root, discovery.Index, discovery.GoModules)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := fileanalyzer.NewRegistry(javascript.New(), pythonanalyzer.New(), goanalyzer.New(moduleIndex))
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	for _, path := range discovery.Paths {
		store.AddNode(Node{Path: path})
	}
	for _, diagnostic := range moduleDiagnostics {
		store.AddDiagnostic(Diagnostic{Path: diagnostic.Path, Message: diagnostic.Message})
	}
	handler, err := NewHandler(root, discovery.Index, registry, store)
	if err != nil {
		t.Fatal(err)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range discovery.Paths {
		queued, err := NewJob(absoluteRoot, path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := handler.Handle(context.Background(), queued); err != nil {
			t.Fatalf("analyze %q: %v", path, err)
		}
	}

	got, err := MarshalReport(GenerateReport("compatibility", store))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(root, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("compatibility graph changed:\n got:\n%s\nwant:\n%s", got, want)
	}
}
