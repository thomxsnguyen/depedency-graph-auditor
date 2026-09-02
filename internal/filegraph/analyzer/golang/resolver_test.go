package golang_test

import (
	"reflect"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/golang"
)

func TestResolve(t *testing.T) {
	root := t.TempDir()
	writeGoFixture(t, root, "go.mod", "module example.com/project\n")
	repository := filegraph.NewRepositoryIndex([]string{
		"root.go",
		"root_test.go",
		"internal/config/load.go",
		"internal/config/types.go",
		"internal/config/config_test.go",
	})
	modules, _, err := golang.BuildModuleIndex(root, repository, []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		importPath string
		want       golang.Resolution
	}{
		{name: "root package", importPath: "example.com/project", want: golang.Resolution{Targets: []string{"root.go"}, Local: true}},
		{name: "nested package", importPath: "example.com/project/internal/config", want: golang.Resolution{Targets: []string{"internal/config/load.go", "internal/config/types.go"}, Local: true}},
		{name: "missing local package", importPath: "example.com/project/internal/missing", want: golang.Resolution{Local: true}},
		{name: "repository escape", importPath: "example.com/project/../outside", want: golang.Resolution{Local: true}},
		{name: "module prefix boundary", importPath: "example.com/project-other/internal/config"},
		{name: "standard library", importPath: "fmt"},
		{name: "external module", importPath: "github.com/example/dependency"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := golang.Resolve(modules, test.importPath); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("resolution: got %+v, want %+v", got, test.want)
			}
		})
	}
}
