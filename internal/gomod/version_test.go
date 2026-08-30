package gomod

import (
	"strings"
	"testing"
)

func TestValidateCoordinateAcceptsGoModuleVersions(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		version    string
	}{
		{name: "release", modulePath: "example.com/module", version: "v1.6.0"},
		{name: "prerelease", modulePath: "example.com/module", version: "v1.6.0-rc.1"},
		{name: "v2 path", modulePath: "example.com/module/v2", version: "v2.3.0"},
		{name: "pseudo-version", modulePath: "example.com/module", version: "v0.0.0-20240102150405-abcdefabcdef"},
		{name: "pseudo-version after tag", modulePath: "example.com/module", version: "v1.2.4-0.20240102150405-abcdefabcdef"},
		{name: "incompatible", modulePath: "example.com/module", version: "v2.3.4+incompatible"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCoordinate(tt.modulePath, tt.version); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateCoordinateRejectsInvalidCoordinates(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		version    string
		wantErr    string
	}{
		{name: "empty", modulePath: "example.com/module", wantErr: "invalid Go module version"},
		{name: "missing v", modulePath: "example.com/module", version: "1.2.3", wantErr: "invalid Go module version"},
		{name: "short version", modulePath: "example.com/module", version: "v1.2", wantErr: "must be canonical"},
		{name: "npm range", modulePath: "example.com/module", version: "^1.2.3", wantErr: "invalid Go module version"},
		{name: "Python constraint", modulePath: "example.com/module", version: ">=1.2", wantErr: "invalid Go module version"},
		{name: "branch", modulePath: "example.com/module", version: "main", wantErr: "invalid Go module version"},
		{name: "tag without canonical version", modulePath: "example.com/module", version: "release-1", wantErr: "invalid Go module version"},
		{name: "invalid path", modulePath: "module", version: "v1.2.3", wantErr: "malformed module path"},
		{name: "major mismatch", modulePath: "example.com/module/v2", version: "v1.2.3", wantErr: "should be v2"},
		{name: "invalid pseudo timestamp", modulePath: "example.com/module", version: "v0.0.0-20241302150405-abcdefabcdef", wantErr: "invalid Go pseudo-version"},
		{name: "invalid pseudo base", modulePath: "example.com/module", version: "v1.0.0-0.20240102150405-abcdefabcdef", wantErr: "invalid Go pseudo-version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCoordinate(tt.modulePath, tt.version)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCompareVersionsUsesGoModuleOrdering(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		want        int
	}{
		{name: "numeric components", left: "v1.9.0", right: "v1.10.0", want: -1},
		{name: "prerelease", left: "v1.0.0-rc.1", right: "v1.0.0", want: -1},
		{name: "pseudo timestamp", left: "v0.0.0-20240101120000-abcdefabcdef", right: "v0.0.0-20240201120000-abcdefabcdef", want: -1},
		{name: "equal", left: "v1.6.0", right: "v1.6.0", want: 0},
		{name: "greater", left: "v2.0.0", right: "v1.99.0", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comparison, err := CompareVersions(tt.left, tt.right)
			if err != nil {
				t.Fatal(err)
			}
			if comparison != tt.want {
				t.Fatalf("comparison: got %d, want %d", comparison, tt.want)
			}
		})
	}
}

func TestCompareVersionsRejectsInvalidInput(t *testing.T) {
	if _, err := CompareVersions("v1.2", "v1.2.3"); err == nil || !strings.Contains(err.Error(), "left version") {
		t.Fatalf("left error: got %v", err)
	}
	if _, err := CompareVersions("v1.2.3", "latest"); err == nil || !strings.Contains(err.Error(), "right version") {
		t.Fatalf("right error: got %v", err)
	}
}
