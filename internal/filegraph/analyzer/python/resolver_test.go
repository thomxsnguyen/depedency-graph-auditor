package python_test

import (
	"reflect"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/python"
)

func TestResolve(t *testing.T) {
	index := filegraph.NewRepositoryIndex([]string{
		"package_mac.py",
		"src/pc_diagnostic/__init__.py",
		"src/pc_diagnostic/models.py",
		"src/pc_diagnostic/alerts/__init__.py",
		"src/pc_diagnostic/alerts/models.py",
		"src/pc_diagnostic/gui/__init__.py",
		"src/pc_diagnostic/gui/app.py",
		"src/pc_diagnostic/gui/bridge.py",
		"src/pc_diagnostic/gui/helpers.py",
		"src/pc_diagnostic/providers/__init__.py",
		"src/pc_diagnostic/providers/base.py",
	})
	tests := []struct {
		name     string
		importer string
		imported python.Import
		want     []string
		local    bool
	}{
		{name: "src absolute module", importer: "tests/test_models.py", imported: python.Import{Module: "pc_diagnostic.models"}, want: []string{"src/pc_diagnostic/models.py"}, local: true},
		{name: "src absolute package", importer: "tests/test_provider.py", imported: python.Import{Module: "pc_diagnostic.providers"}, want: []string{"src/pc_diagnostic/providers/__init__.py"}, local: true},
		{name: "module imported from absolute package", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Module: "pc_diagnostic", Names: []string{"models"}}, want: []string{"src/pc_diagnostic/models.py"}, local: true},
		{name: "root module", importer: "tests/test_packaging.py", imported: python.Import{Module: "package_mac"}, want: []string{"package_mac.py"}, local: true},
		{name: "relative sibling", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Module: "bridge", Level: 1}, want: []string{"src/pc_diagnostic/gui/bridge.py"}, local: true},
		{name: "relative parent", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Module: "models", Level: 2}, want: []string{"src/pc_diagnostic/models.py"}, local: true},
		{name: "relative imported names", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Level: 1, Names: []string{"helpers"}}, want: []string{"src/pc_diagnostic/gui/helpers.py"}, local: true},
		{name: "relative attribute falls back to package", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Level: 1, Names: []string{"CONSTANT"}}, want: []string{"src/pc_diagnostic/gui/__init__.py"}, local: true},
		{name: "external standard library", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Module: "os"}},
		{name: "external package", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Module: "psutil"}},
		{name: "unresolved local", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Module: "pc_diagnostic.missing"}, local: true},
		{name: "relative root escape", importer: "src/pc_diagnostic/gui/app.py", imported: python.Import{Module: "missing", Level: 8}, local: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, local := python.Resolve(index, test.importer, test.imported)
			if !reflect.DeepEqual(got, test.want) || local != test.local {
				t.Fatalf("Resolve: got (%v, %t), want (%v, %t)", got, local, test.want, test.local)
			}
		})
	}
}
