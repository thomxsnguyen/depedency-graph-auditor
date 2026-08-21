// Package depfile parses package.json files and extracts the dependency map.
// It is the seed step for the auditor — the entry point reads a package.json
// and uses this package to produce the initial set of jobs to enqueue.
package depfile

import (
	"encoding/json"
	"fmt"
	"os"
)

// Dependency represents a single entry from a dependency file.
type Dependency struct {
	Name         string
	VersionRange string // raw range from the file, e.g., "^4.18.0"
}

// packageJSON is the subset of package.json fields we care about.
type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// ParsePackageJSON reads a package.json file at path and returns its
// dependencies. Both "dependencies" and "devDependencies" are included by
// default — a license violation in a dev dependency matters legally because the
// package is still downloaded and used.
//
// Set includeDevDeps to false to restrict the result to production dependencies
// only (the "dependencies" key).
func ParsePackageJSON(path string, includeDevDeps bool) ([]Dependency, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("depfile: open %s: %w", path, err)
	}
	defer f.Close()

	var pkg packageJSON
	if err := json.NewDecoder(f).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("depfile: decode %s: %w", path, err)
	}

	var deps []Dependency

	for name, versionRange := range pkg.Dependencies {
		deps = append(deps, Dependency{Name: name, VersionRange: versionRange})
	}

	if includeDevDeps {
		for name, versionRange := range pkg.DevDependencies {
			deps = append(deps, Dependency{Name: name, VersionRange: versionRange})
		}
	}

	return deps, nil
}
