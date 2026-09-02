package golang_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/golang"
)

func TestBuildModuleIndex(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module example.com/project\n\ngo 1.26.6\n")
	index := filegraph.NewRepositoryIndex([]string{
		"main.go",
		"internal/config/load.go",
		"internal/config/types.go",
		"internal/config/config_test.go",
		"nested/ignored.go",
		"vendor/example.com/dependency/ignored.go",
	})
	modules, diagnostics, err := golang.BuildModuleIndex(root, index, []string{"nested/go.mod", "go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if modules.ModulePath() != "example.com/project" {
		t.Fatalf("module path: %q", modules.ModulePath())
	}
	wantFiles := []string{"internal/config/load.go", "internal/config/types.go"}
	if got := modules.PackageFiles("internal/config"); !reflect.DeepEqual(got, wantFiles) {
		t.Fatalf("package files: got %v, want %v", got, wantFiles)
	}
	if !modules.Owns("internal/config/load.go") || modules.Owns("nested/ignored.go") {
		t.Fatalf("unexpected module ownership")
	}
	if len(diagnostics) != 1 || diagnostics[0].Path != "nested/go.mod" {
		t.Fatalf("diagnostics: %+v", diagnostics)
	}

	files := modules.PackageFiles("internal/config")
	files[0] = "mutated.go"
	if got := modules.PackageFiles("internal/config"); !reflect.DeepEqual(got, wantFiles) {
		t.Fatalf("package file mutation escaped: %v", got)
	}
}

func TestBuildModuleIndexReportsMissingAndInvalidRootModule(t *testing.T) {
	index := filegraph.NewRepositoryIndex([]string{"main.go"})

	missing, diagnostics, err := golang.BuildModuleIndex(t.TempDir(), index, nil)
	if err != nil {
		t.Fatal(err)
	}
	if missing.ModulePath() != "" || len(diagnostics) != 1 || diagnostics[0].Path != "go.mod" {
		t.Fatalf("missing module result: module=%q diagnostics=%+v", missing.ModulePath(), diagnostics)
	}

	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module\n")
	invalid, diagnostics, err := golang.BuildModuleIndex(root, index, []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if invalid.ModulePath() != "" || len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "invalid root go.mod") {
		t.Fatalf("invalid module result: module=%q diagnostics=%+v", invalid.ModulePath(), diagnostics)
	}

	rootWithoutDirective := t.TempDir()
	writeGoFixture(t, rootWithoutDirective, "go.mod", "go 1.26.6\n")
	withoutDirective, diagnostics, err := golang.BuildModuleIndex(rootWithoutDirective, index, []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if withoutDirective.ModulePath() != "" || len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "no module directive") {
		t.Fatalf("missing directive result: module=%q diagnostics=%+v", withoutDirective.ModulePath(), diagnostics)
	}
}

func TestBuildModuleIndexSkipsConfigurationWhenRepositoryHasNoGoFiles(t *testing.T) {
	modules, diagnostics, err := golang.BuildModuleIndex(t.TempDir(), filegraph.NewRepositoryIndex([]string{"app.ts"}), nil)
	if err != nil || modules.ModulePath() != "" || len(diagnostics) != 0 {
		t.Fatalf("non-Go repository: module=%q diagnostics=%+v err=%v", modules.ModulePath(), diagnostics, err)
	}
}

func TestModuleIndexSupportsConcurrentReads(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module example.com/project\n")
	index := filegraph.NewRepositoryIndex([]string{"internal/config/load.go"})
	modules, _, err := golang.BuildModuleIndex(root, index, []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	for range 20 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			if modules.ModulePath() != "example.com/project" || len(modules.PackageFiles("internal/config")) != 1 {
				t.Errorf("unexpected concurrent module index read")
			}
		}()
	}
	readers.Wait()
}

func writeGoFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	filePath := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
