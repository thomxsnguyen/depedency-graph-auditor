package filegraph

import (
	"path"
	"sort"
	"strings"
)

// RepositoryIndex is an immutable lookup of normalized repository paths.
type RepositoryIndex struct {
	paths []string
	set   map[string]struct{}
}

// NewRepositoryIndex copies, normalizes, sorts, and deduplicates paths.
func NewRepositoryIndex(paths []string) RepositoryIndex {
	set := make(map[string]struct{}, len(paths))
	for _, candidate := range paths {
		normalized := path.Clean(strings.ReplaceAll(candidate, "\\", "/"))
		if normalized == "." || normalized == ".." || path.IsAbs(normalized) || strings.HasPrefix(normalized, "../") {
			continue
		}
		set[normalized] = struct{}{}
	}
	normalizedPaths := make([]string, 0, len(set))
	for candidate := range set {
		normalizedPaths = append(normalizedPaths, candidate)
	}
	sort.Strings(normalizedPaths)
	return RepositoryIndex{paths: normalizedPaths, set: set}
}

// Has reports whether a normalized repository path exists in the index.
func (i RepositoryIndex) Has(candidate string) bool {
	normalized := path.Clean(strings.ReplaceAll(candidate, "\\", "/"))
	_, exists := i.set[normalized]
	return exists
}

// Paths returns a copy of all indexed paths in deterministic order.
func (i RepositoryIndex) Paths() []string {
	paths := make([]string, len(i.paths))
	copy(paths, i.paths)
	return paths
}
