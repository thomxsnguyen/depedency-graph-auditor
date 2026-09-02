package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSourceEnforcesRootAndSizeLimit(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "app.ts")
	if err := os.WriteFile(valid, []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadSource(root, "app.ts"); err != nil || string(data) != "export {}" {
		t.Fatalf("valid source: data=%q err=%v", data, err)
	}
	if _, err := ReadSource(root, "../outside.ts"); err == nil {
		t.Fatal("expected escaping path error")
	}

	large := filepath.Join(root, "large.ts")
	if err := os.WriteFile(large, []byte(strings.Repeat("x", int(maxSourceFileBytes)+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSource(root, "large.ts"); err == nil {
		t.Fatal("expected size-limit error")
	}
}
