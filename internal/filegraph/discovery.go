package filegraph

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Index is a read-only lookup set of normalized project-relative source paths.
type Index map[string]struct{}

var excludedDirectories = map[string]struct{}{
	".git":          {},
	".mypy_cache":   {},
	".pytest_cache": {},
	".ruff_cache":   {},
	".venv":         {},
	"__pycache__":   {},
	"build":         {},
	"coverage":      {},
	"dist":          {},
	"node_modules":  {},
	"site-packages": {},
	"venv":          {},
}

// Discover finds supported source files beneath root and returns a stable list
// plus an index used by import resolution.
func Discover(root string) ([]string, Index, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, fmt.Errorf("filegraph: resolve project root %q: %w", root, err)
	}
	absoluteRoot = filepath.Clean(absoluteRoot)
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("filegraph: inspect project root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("filegraph: project root %q is not a directory", root)
	}

	paths := make([]string, 0)
	err = filepath.WalkDir(absoluteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absoluteRoot {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if _, excluded := excludedDirectories[entry.Name()]; excluded {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !supportedExtension(filepath.Ext(entry.Name())) {
			return nil
		}
		relative, err := filepath.Rel(absoluteRoot, path)
		if err != nil {
			return fmt.Errorf("make %q relative to project root: %w", path, err)
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("filegraph: discover source files under %q: %w", root, err)
	}

	sort.Strings(paths)
	index := make(Index, len(paths))
	for _, path := range paths {
		index[path] = struct{}{}
	}
	return paths, index, nil
}

func supportedExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".js", ".jsx", ".ts", ".tsx", ".py":
		return true
	default:
		return false
	}
}
