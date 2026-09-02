package python

import (
	"path"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
)

// Resolve maps an import to repository-local Python source files. The bool
// reports whether an unresolved import belongs to a known local package.
func Resolve(index analyzer.Index, importer string, imported Import) ([]string, bool) {
	if imported.Level > 0 {
		return resolveRelative(index, importer, imported)
	}
	base := strings.ReplaceAll(imported.Module, ".", "/")
	for _, candidateBase := range []string{base, path.Join("src", base)} {
		if resolved := candidate(index, candidateBase); resolved != "" {
			if strings.HasSuffix(resolved, "/__init__.py") {
				if modules := importedModules(index, candidateBase, imported.Names); len(modules) > 0 {
					return modules, true
				}
			}
			return []string{resolved}, true
		}
	}
	return nil, topLevelIsLocal(index, imported.Module)
}

func importedModules(index analyzer.Index, packageBase string, names []string) []string {
	resolved := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "*" {
			continue
		}
		module := candidate(index, path.Join(packageBase, name))
		if module == "" {
			continue
		}
		if _, exists := seen[module]; exists {
			continue
		}
		seen[module] = struct{}{}
		resolved = append(resolved, module)
	}
	return resolved
}

func resolveRelative(index analyzer.Index, importer string, imported Import) ([]string, bool) {
	base := path.Dir(importer)
	for level := 1; level < imported.Level; level++ {
		base = path.Dir(base)
		if base == "." || base == ".." || strings.HasPrefix(base, "../") {
			return nil, true
		}
	}
	if imported.Module != "" {
		moduleBase := path.Join(base, strings.ReplaceAll(imported.Module, ".", "/"))
		if resolved := candidate(index, moduleBase); resolved != "" {
			return []string{resolved}, true
		}
		return nil, true
	}

	resolved := make([]string, 0, len(imported.Names))
	for _, name := range imported.Names {
		if name == "*" {
			continue
		}
		if module := candidate(index, path.Join(base, name)); module != "" {
			resolved = append(resolved, module)
		}
	}
	if len(resolved) > 0 {
		return resolved, true
	}
	if packageInit := path.Join(base, "__init__.py"); hasPath(index, packageInit) {
		return []string{packageInit}, true
	}
	return nil, true
}

func candidate(index analyzer.Index, base string) string {
	for _, possible := range []string{base + ".py", path.Join(base, "__init__.py")} {
		if hasPath(index, possible) {
			return possible
		}
	}
	return ""
}

func topLevelIsLocal(index analyzer.Index, module string) bool {
	topLevel := strings.Split(module, ".")[0]
	if topLevel == "" {
		return false
	}
	for _, possible := range []string{
		topLevel + ".py",
		path.Join(topLevel, "__init__.py"),
		path.Join("src", topLevel, "__init__.py"),
	} {
		if hasPath(index, possible) {
			return true
		}
	}
	return false
}

func hasPath(index analyzer.Index, possible string) bool {
	if possible == "." || possible == ".." || strings.HasPrefix(possible, "../") || path.IsAbs(possible) {
		return false
	}
	return index.Has(possible)
}
