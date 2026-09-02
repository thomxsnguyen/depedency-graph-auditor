package golang

import (
	"reflect"
	"testing"
)

func TestExtractImports(t *testing.T) {
	source := []byte(`package service

import "example.com/project/internal/config"
import (
	alias "example.com/project/internal/service"
	_ "example.com/project/internal/register"
	. ` + "`example.com/project/internal/helpers`" + `
	"fmt"
)

var text = "import \"example.com/project/ignored\""
// import "example.com/project/comment"
`)
	got, err := ExtractImports("service.go", source)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"example.com/project/internal/config",
		"example.com/project/internal/service",
		"example.com/project/internal/register",
		"example.com/project/internal/helpers",
		"fmt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports: got %v, want %v", got, want)
	}
}

func TestExtractImportsRejectsMalformedSource(t *testing.T) {
	if _, err := ExtractImports("broken.go", []byte("package broken\nimport (\n\"fmt\"")); err == nil {
		t.Fatal("expected parser error")
	}
}
