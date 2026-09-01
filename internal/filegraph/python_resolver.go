package filegraph

import (
	"path"
	"strings"
)

// ResolvePython maps an import to repository-local Python source files. The
// bool reports whether an unresolved import belongs to a known local package.
func ResolvePython(index Index, importer string, imported PythonImport) ([]string, bool) {
	if imported.Level > 0 {
		return resolveRelativePython(index, importer, imported)
	}
	base := strings.ReplaceAll(imported.Module, ".", "/")
	for _, candidateBase := range []string{base, path.Join("src", base)} {
		if resolved := pythonCandidate(index, candidateBase); resolved != "" {
			if strings.HasSuffix(resolved, "/__init__.py") {
				if modules := pythonImportedModules(index, candidateBase, imported.Names); len(modules) > 0 {
					return modules, true
				}
			}
			return []string{resolved}, true
		}
	}
	return nil, pythonTopLevelIsLocal(index, imported.Module)
}

func pythonImportedModules(index Index, packageBase string, names []string) []string {
	resolved := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "*" {
			continue
		}
		candidate := pythonCandidate(index, path.Join(packageBase, name))
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		resolved = append(resolved, candidate)
	}
	return resolved
}

func resolveRelativePython(index Index, importer string, imported PythonImport) ([]string, bool) {
	base := path.Dir(importer)
	for level := 1; level < imported.Level; level++ {
		base = path.Dir(base)
		if base == "." || base == ".." || strings.HasPrefix(base, "../") {
			return nil, true
		}
	}
	if imported.Module != "" {
		moduleBase := path.Join(base, strings.ReplaceAll(imported.Module, ".", "/"))
		if resolved := pythonCandidate(index, moduleBase); resolved != "" {
			return []string{resolved}, true
		}
		return nil, true
	}

	resolved := make([]string, 0, len(imported.Names))
	for _, name := range imported.Names {
		if name == "*" {
			continue
		}
		if candidate := pythonCandidate(index, path.Join(base, name)); candidate != "" {
			resolved = append(resolved, candidate)
		}
	}
	if len(resolved) > 0 {
		return resolved, true
	}
	if packageInit := path.Join(base, "__init__.py"); hasIndexedPath(index, packageInit) {
		return []string{packageInit}, true
	}
	return nil, true
}

func pythonCandidate(index Index, base string) string {
	for _, candidate := range []string{base + ".py", path.Join(base, "__init__.py")} {
		if hasIndexedPath(index, candidate) {
			return candidate
		}
	}
	return ""
}

func pythonTopLevelIsLocal(index Index, module string) bool {
	topLevel := strings.Split(module, ".")[0]
	if topLevel == "" {
		return false
	}
	for _, candidate := range []string{
		topLevel + ".py",
		path.Join(topLevel, "__init__.py"),
		path.Join("src", topLevel, "__init__.py"),
	} {
		if hasIndexedPath(index, candidate) {
			return true
		}
	}
	return false
}

func hasIndexedPath(index Index, candidate string) bool {
	if candidate == "." || candidate == ".." || strings.HasPrefix(candidate, "../") || path.IsAbs(candidate) {
		return false
	}
	_, exists := index[candidate]
	return exists
}
