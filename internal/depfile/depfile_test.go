package depfile_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// writeTempPackageJSON writes a package.json with the given content to a
// temporary file and returns its path. The file is automatically cleaned up
// when the test ends.
func writeTempPackageJSON(t *testing.T, content any) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "package-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := json.NewEncoder(f).Encode(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

// depMap turns a []Dependency into a name→versionRange map for easier assertions.
func depMap(deps []depfile.Dependency) map[string]string {
	m := make(map[string]string, len(deps))
	for _, d := range deps {
		m[d.Name] = d.VersionRange
	}
	return m
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestParsePackageJSONProductionDeps verifies that "dependencies" entries are
// returned with the correct name and version range.
func TestParsePackageJSONProductionDeps(t *testing.T) {
	path := writeTempPackageJSON(t, map[string]any{
		"name": "my-app",
		"dependencies": map[string]string{
			"express": "^4.18.0",
			"lodash":  "~4.17.0",
		},
	})

	manifest, err := depfile.ParsePackageJSONFile(path, false)
	if err != nil {
		t.Fatalf("ParsePackageJSON: unexpected error: %v", err)
	}
	if manifest.Name != "my-app" {
		t.Fatalf("name: got %q, want %q", manifest.Name, "my-app")
	}
	if len(manifest.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(manifest.Dependencies))
	}

	m := depMap(manifest.Dependencies)
	if m["express"] != "^4.18.0" {
		t.Errorf("express version range: got %q, want %q", m["express"], "^4.18.0")
	}
	if m["lodash"] != "~4.17.0" {
		t.Errorf("lodash version range: got %q, want %q", m["lodash"], "~4.17.0")
	}
}

// TestParsePackageJSONDevDepsIncluded verifies that when includeDevDeps=true,
// both "dependencies" and "devDependencies" entries are returned.
func TestParsePackageJSONDevDepsIncluded(t *testing.T) {
	path := writeTempPackageJSON(t, map[string]any{
		"name": "my-app",
		"dependencies": map[string]string{
			"express": "^4.18.0",
		},
		"devDependencies": map[string]string{
			"jest": "^29.0.0",
		},
	})

	manifest, err := depfile.ParsePackageJSONFile(path, true)
	if err != nil {
		t.Fatalf("ParsePackageJSON: unexpected error: %v", err)
	}
	if len(manifest.Dependencies) != 2 {
		t.Fatalf("expected 2 deps (prod + dev), got %d", len(manifest.Dependencies))
	}

	m := depMap(manifest.Dependencies)
	if m["jest"] != "^29.0.0" {
		t.Errorf("jest (devDep) version range: got %q, want %q", m["jest"], "^29.0.0")
	}
}

// TestParsePackageJSONDevDepsExcluded verifies that when includeDevDeps=false,
// "devDependencies" entries are not returned.
func TestParsePackageJSONDevDepsExcluded(t *testing.T) {
	path := writeTempPackageJSON(t, map[string]any{
		"name": "my-app",
		"dependencies": map[string]string{
			"express": "^4.18.0",
		},
		"devDependencies": map[string]string{
			"jest": "^29.0.0",
		},
	})

	manifest, err := depfile.ParsePackageJSONFile(path, false)
	if err != nil {
		t.Fatalf("ParsePackageJSON: unexpected error: %v", err)
	}
	if len(manifest.Dependencies) != 1 {
		t.Fatalf("expected 1 prod dep only, got %d", len(manifest.Dependencies))
	}
	m := depMap(manifest.Dependencies)
	if _, ok := m["jest"]; ok {
		t.Error("jest (devDep) should not be present when includeDevDeps=false")
	}
}

// TestParsePackageJSONMissingDependenciesKey verifies that a package.json with
// no "dependencies" key returns an empty slice without error.
func TestParsePackageJSONMissingDependenciesKey(t *testing.T) {
	path := writeTempPackageJSON(t, map[string]any{
		"name": "my-app",
	})

	manifest, err := depfile.ParsePackageJSONFile(path, false)
	if err != nil {
		t.Fatalf("ParsePackageJSON: unexpected error: %v", err)
	}
	if len(manifest.Dependencies) != 0 {
		t.Errorf("expected 0 deps for missing key, got %d", len(manifest.Dependencies))
	}
}

// TestParsePackageJSONMissingDevDependenciesKey verifies that includeDevDeps=true
// on a file with no "devDependencies" key still returns only prod deps without error.
func TestParsePackageJSONMissingDevDependenciesKey(t *testing.T) {
	path := writeTempPackageJSON(t, map[string]any{
		"name": "my-app",
		"dependencies": map[string]string{
			"express": "^4.18.0",
		},
	})

	manifest, err := depfile.ParsePackageJSONFile(path, true)
	if err != nil {
		t.Fatalf("ParsePackageJSON: unexpected error: %v", err)
	}
	if len(manifest.Dependencies) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(manifest.Dependencies))
	}
}

// TestParsePackageJSONFileNotFound verifies that a non-existent path returns an error.
func TestParsePackageJSONFileNotFound(t *testing.T) {
	_, err := depfile.ParsePackageJSONFile("/does/not/exist/package.json", false)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// TestParsePackageJSONInvalidJSON verifies that malformed JSON returns an error.
func TestParsePackageJSONInvalidJSON(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "package-*.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	f.WriteString("not valid json{{{")
	f.Close()

	_, err = depfile.ParsePackageJSONFile(f.Name(), false)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParsePackageJSONReaderIsDeterministic(t *testing.T) {
	manifest, err := depfile.ParsePackageJSON(strings.NewReader(`{
		"name":"reader-app",
		"dependencies":{"zeta":"2.0.0","alpha":"1.0.0"},
		"devDependencies":{"middle":"3.0.0"}
	}`), true)
	if err != nil {
		t.Fatal(err)
	}

	want := depfile.Manifest{
		Name: "reader-app",
		Dependencies: []depfile.Dependency{
			{Name: "alpha", VersionRange: "1.0.0"},
			{Name: "middle", VersionRange: "3.0.0"},
			{Name: "zeta", VersionRange: "2.0.0"},
		},
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("manifest: got %+v, want %+v", manifest, want)
	}
}
