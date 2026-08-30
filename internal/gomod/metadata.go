// Package gomod retrieves and parses public Go module metadata.
package gomod

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Requirement is one exact module requirement from a dependency go.mod.
type Requirement struct {
	ModulePath string
	Version    string
	Indirect   bool
}

// Metadata is the traversal-relevant subset of a dependency go.mod.
type Metadata struct {
	ModulePath   string
	GoVersion    string
	Requirements []Requirement
}

// ParseMetadata parses a non-main module with Go's forward-compatible lax
// parser. Dependency replace and exclude directives are intentionally ignored.
func ParseMetadata(data []byte) (Metadata, error) {
	file, err := modfile.ParseLax("go.mod", data, preserveVersion)
	if err != nil {
		return Metadata{}, fmt.Errorf("parse dependency go.mod: %w", err)
	}
	if file.Module == nil || strings.TrimSpace(file.Module.Mod.Path) == "" {
		return Metadata{}, fmt.Errorf("dependency go.mod requires a module directive")
	}
	modulePath := file.Module.Mod.Path
	if err := module.CheckPath(modulePath); err != nil {
		return Metadata{}, fmt.Errorf("dependency go.mod module %q: %w", modulePath, err)
	}

	requirements := make([]Requirement, 0, len(file.Require))
	for _, requirement := range file.Require {
		path := requirement.Mod.Path
		version := requirement.Mod.Version
		if err := ValidateCoordinate(path, version); err != nil {
			return Metadata{}, fmt.Errorf("dependency go.mod requirement %s@%s: %w", path, version, err)
		}
		requirements = append(requirements, Requirement{
			ModulePath: path,
			Version:    version,
			Indirect:   requirement.Indirect,
		})
	}
	sort.SliceStable(requirements, func(i, j int) bool {
		if requirements[i].ModulePath != requirements[j].ModulePath {
			return requirements[i].ModulePath < requirements[j].ModulePath
		}
		comparison := compareCanonicalVersions(requirements[i].Version, requirements[j].Version)
		if comparison != 0 {
			return comparison < 0
		}
		return requirements[i].Version < requirements[j].Version
	})

	metadata := Metadata{ModulePath: modulePath, Requirements: requirements}
	if file.Go != nil {
		metadata.GoVersion = file.Go.Version
	}
	return metadata, nil
}

func preserveVersion(_ string, version string) (string, error) {
	return version, nil
}
