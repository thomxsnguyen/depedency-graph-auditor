package javascript_test

import (
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/javascript"
)

func TestResolve(t *testing.T) {
	index := filegraph.NewRepositoryIndex([]string{
		"src/exact.ts",
		"src/component.tsx",
		"src/util.js",
		"src/legacy.jsx",
		"src/folder/index.ts",
		"shared/index.tsx",
		"src/preferred.ts",
		"src/preferred.tsx",
	})
	tests := []struct {
		name      string
		importer  string
		specifier string
		want      string
		found     bool
	}{
		{name: "exact", importer: "src/App.tsx", specifier: "./exact.ts", want: "src/exact.ts", found: true},
		{name: "TSX extension", importer: "src/App.tsx", specifier: "./component", want: "src/component.tsx", found: true},
		{name: "JS extension", importer: "src/App.tsx", specifier: "./util", want: "src/util.js", found: true},
		{name: "JSX extension", importer: "src/App.tsx", specifier: "./legacy", want: "src/legacy.jsx", found: true},
		{name: "directory index", importer: "src/App.tsx", specifier: "./folder", want: "src/folder/index.ts", found: true},
		{name: "parent directory", importer: "src/nested/App.tsx", specifier: "../../shared", want: "shared/index.tsx", found: true},
		{name: "extension priority", importer: "src/App.tsx", specifier: "./preferred", want: "src/preferred.ts", found: true},
		{name: "external", importer: "src/App.tsx", specifier: "react"},
		{name: "root escape", importer: "src/App.tsx", specifier: "../../secret"},
		{name: "missing", importer: "src/App.tsx", specifier: "./missing"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := javascript.Resolve(index, test.importer, test.specifier)
			if got != test.want || found != test.found {
				t.Fatalf("Resolve: got (%q, %t), want (%q, %t)", got, found, test.want, test.found)
			}
		})
	}
}
