package filegraph

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestDiscoverFiltersAndSortsSourceFiles(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/z.js", "")
	writeFixture(t, root, "src/a.tsx", "")
	writeFixture(t, root, "src/python.py", "")
	writeFixture(t, root, "cmd/server/main.go", "package main")
	writeFixture(t, root, "go.mod", "module example.com/project")
	writeFixture(t, root, "nested/go.mod", "module example.com/nested")
	writeFixture(t, root, "src/readme.md", "")
	writeFixture(t, root, "node_modules/package/index.js", "")
	writeFixture(t, root, "dist/output.js", "")
	writeFixture(t, root, "build/output.js", "")
	writeFixture(t, root, "coverage/output.js", "")
	writeFixture(t, root, ".git/hooks/test.js", "")
	writeFixture(t, root, ".venv/lib/site-packages/dependency.py", "")
	writeFixture(t, root, "venv/dependency.py", "")
	writeFixture(t, root, "src/__pycache__/cached.py", "")
	writeFixture(t, root, ".pytest_cache/cached.py", "")
	writeFixture(t, root, ".mypy_cache/cached.py", "")
	writeFixture(t, root, ".ruff_cache/cached.py", "")
	writeFixture(t, root, "vendor/example.com/dependency/ignored.go", "package dependency")

	discovery, err := DiscoverRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cmd/server/main.go", "src/a.tsx", "src/python.py", "src/z.js"}
	if !reflect.DeepEqual(discovery.Paths, want) {
		t.Fatalf("paths: got %v, want %v", discovery.Paths, want)
	}
	for _, path := range want {
		if !discovery.Index.Has(path) {
			t.Fatalf("index omitted %q", path)
		}
	}
	wantModules := []string{"go.mod", "nested/go.mod"}
	if !reflect.DeepEqual(discovery.GoModules, wantModules) {
		t.Fatalf("Go modules: got %v, want %v", discovery.GoModules, wantModules)
	}
	if discovery.Index.Has("go.mod") {
		t.Fatal("go.mod must not become a source node")
	}
}

func TestDiscoverSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, outside, "outside.ts", "")
	if err := os.Symlink(filepath.Join(outside, "outside.ts"), filepath.Join(root, "linked.ts")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}

	paths, _, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("paths: got %v, want none", paths)
	}
}

func TestDiscoverRejectsNonDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "App.ts")
	writeFixture(t, root, "App.ts", "")
	if _, _, err := Discover(path); err == nil {
		t.Fatal("expected non-directory error")
	}
}

func writeFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
