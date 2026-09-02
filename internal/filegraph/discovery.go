package filegraph

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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
	"vendor":        {},
	"venv":          {},
}

// Discovery is one deterministic, single-pass repository inventory.
type Discovery struct {
	Paths     []string
	Index     RepositoryIndex
	GoModules []string
}

// Discover finds supported source files beneath root and returns a stable list
// plus an index used by import resolution.
func Discover(root string) ([]string, RepositoryIndex, error) {
	discovery, err := DiscoverRepository(root)
	if err != nil {
		return nil, RepositoryIndex{}, err
	}
	return discovery.Paths, discovery.Index, nil
}

// DiscoverRepository finds supported sources and Go module boundaries in one
// repository walk. Go manifests are metadata and never source graph nodes.
func DiscoverRepository(root string) (Discovery, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Discovery{}, fmt.Errorf("filegraph: resolve project root %q: %w", root, err)
	}
	absoluteRoot = filepath.Clean(absoluteRoot)
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return Discovery{}, fmt.Errorf("filegraph: inspect project root %q: %w", root, err)
	}
	if !info.IsDir() {
		return Discovery{}, fmt.Errorf("filegraph: project root %q is not a directory", root)
	}

	paths := make([]string, 0)
	goModules := make([]string, 0)
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
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(absoluteRoot, path)
		if err != nil {
			return fmt.Errorf("make %q relative to project root: %w", path, err)
		}
		relative = filepath.ToSlash(relative)
		if entry.Name() == "go.mod" {
			goModules = append(goModules, relative)
		}
		if supportedExtension(filepath.Ext(entry.Name())) {
			paths = append(paths, relative)
		}
		return nil
	})
	if err != nil {
		return Discovery{}, fmt.Errorf("filegraph: discover source files under %q: %w", root, err)
	}

	index := NewRepositoryIndex(paths)
	moduleIndex := NewRepositoryIndex(goModules)
	return Discovery{Paths: index.Paths(), Index: index, GoModules: moduleIndex.Paths()}, nil
}

func supportedExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py":
		return true
	default:
		return false
	}
}
