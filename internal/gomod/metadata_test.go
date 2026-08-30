package gomod

import (
	"strings"
	"testing"
)

func TestParseMetadataUsesLaxDependencyRules(t *testing.T) {
	metadata, err := ParseMetadata([]byte(`
module example.com/dependency
go 1.22
future-directive ignored
require example.com/zeta v1.10.0
require (
	example.com/alpha v0.3.0 // indirect
	example.com/zeta v1.9.0
)
replace example.com/alpha => ../local-alpha
exclude example.com/zeta v1.8.0
`))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ModulePath != "example.com/dependency" || metadata.GoVersion != "1.22" {
		t.Fatalf("metadata: %+v", metadata)
	}
	want := []Requirement{
		{ModulePath: "example.com/alpha", Version: "v0.3.0", Indirect: true},
		{ModulePath: "example.com/zeta", Version: "v1.9.0"},
		{ModulePath: "example.com/zeta", Version: "v1.10.0"},
	}
	if len(metadata.Requirements) != len(want) {
		t.Fatalf("requirements: got %+v, want %+v", metadata.Requirements, want)
	}
	for index := range want {
		if metadata.Requirements[index] != want[index] {
			t.Fatalf("requirement %d: got %+v, want %+v", index, metadata.Requirements[index], want[index])
		}
	}
}

func TestParseMetadataAllowsMissingGoDirective(t *testing.T) {
	metadata, err := ParseMetadata([]byte("module example.com/legacy\n"))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.GoVersion != "" {
		t.Fatalf("Go version: got %q, want empty", metadata.GoVersion)
	}
}

func TestParseMetadataRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "syntax", content: "module (\n", wantErr: "parse dependency go.mod"},
		{name: "missing module", content: "go 1.22\n", wantErr: "requires a module directive"},
		{name: "invalid module", content: "module dependency\n", wantErr: "malformed module path"},
		{name: "noncanonical requirement", content: "module example.com/dependency\nrequire example.com/a v1.2\n", wantErr: "version must be canonical"},
		{name: "major mismatch", content: "module example.com/dependency\nrequire example.com/a/v2 v1.2.3\n", wantErr: "should be v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMetadata([]byte(tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
