package python_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	fileanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/python"
)

func TestAnalyzerReturnsNormalizedDependenciesAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "src/widget/__init__.py", "")
	writeSource(t, root, "src/widget/main.py", "from widget import models\nfrom widget.missing import Missing\nimport requests\n")
	writeSource(t, root, "src/widget/models.py", "")
	index := filegraph.NewRepositoryIndex([]string{
		"src/widget/__init__.py", "src/widget/main.py", "src/widget/models.py",
	})
	result, err := python.New().Analyze(context.Background(), fileanalyzer.FileContext{
		Root: root, Path: "src/widget/main.py", Index: index,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDependencies := []fileanalyzer.Dependency{{Target: "src/widget/models.py", Kind: "import", Confidence: "exact"}}
	if !reflect.DeepEqual(result.Dependencies, wantDependencies) {
		t.Fatalf("dependencies: got %+v, want %+v", result.Dependencies, wantDependencies)
	}
	wantDiagnostics := []fileanalyzer.Diagnostic{{Reference: "widget.missing", Message: "unresolved local import"}}
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
