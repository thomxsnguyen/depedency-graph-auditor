// Package golang analyzes Go file dependencies within one root module.
package golang

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	"golang.org/x/mod/modfile"
)

// ModuleDiagnostic reports deterministic Go module configuration limitations.
type ModuleDiagnostic struct {
	Path    string
	Message string
}

// ModuleIndex is an immutable mapping from local package directories to the
// ordinary Go source files that compile into them.
type ModuleIndex struct {
	modulePath   string
	packageFiles map[string][]string
	nestedRoots  []string
}

// BuildModuleIndex builds Go package metadata from the shared repository index
// without walking the repository again.
func BuildModuleIndex(root string, index analyzer.Index, moduleFiles []string) (ModuleIndex, []ModuleDiagnostic, error) {
	result := ModuleIndex{packageFiles: make(map[string][]string)}
	if index == nil {
		return result, nil, fmt.Errorf("filegraph: repository index is required")
	}

	paths := index.Paths()
	hasGoFiles := false
	for _, sourcePath := range paths {
		if strings.EqualFold(path.Ext(sourcePath), ".go") {
			hasGoFiles = true
			break
		}
	}
	if !hasGoFiles {
		return result, nil, nil
	}

	modules := normalizePaths(moduleFiles)
	hasRootModule := false
	diagnostics := make([]ModuleDiagnostic, 0)
	for _, modulePath := range modules {
		if modulePath == "go.mod" {
			hasRootModule = true
			continue
		}
		rootPath := path.Dir(modulePath)
		result.nestedRoots = append(result.nestedRoots, rootPath)
		diagnostics = append(diagnostics, ModuleDiagnostic{
			Path:    modulePath,
			Message: "nested Go modules are not supported",
		})
	}
	result.nestedRoots = normalizePaths(result.nestedRoots)

	if !hasRootModule {
		diagnostics = append(diagnostics, ModuleDiagnostic{
			Path:    "go.mod",
			Message: "root go.mod is required for Go file dependency resolution",
		})
		sortModuleDiagnostics(diagnostics)
		return result, diagnostics, nil
	}

	data, err := analyzer.ReadSource(root, "go.mod")
	if err != nil {
		return result, nil, fmt.Errorf("filegraph: read root go.mod: %w", err)
	}
	parsed, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		diagnostics = append(diagnostics, ModuleDiagnostic{Path: "go.mod", Message: "invalid root go.mod: " + err.Error()})
		sortModuleDiagnostics(diagnostics)
		return result, diagnostics, nil
	}
	if parsed.Module == nil || strings.TrimSpace(parsed.Module.Mod.Path) == "" {
		diagnostics = append(diagnostics, ModuleDiagnostic{Path: "go.mod", Message: "root go.mod has no module directive"})
		sortModuleDiagnostics(diagnostics)
		return result, diagnostics, nil
	}
	result.modulePath = parsed.Module.Mod.Path

	for _, sourcePath := range paths {
		if !strings.EqualFold(path.Ext(sourcePath), ".go") || strings.HasSuffix(strings.ToLower(sourcePath), "_test.go") {
			continue
		}
		if inDirectory(sourcePath, "vendor") || result.inNestedModule(sourcePath) {
			continue
		}
		directory := path.Dir(sourcePath)
		result.packageFiles[directory] = append(result.packageFiles[directory], sourcePath)
	}
	for directory, files := range result.packageFiles {
		result.packageFiles[directory] = normalizePaths(files)
	}
	sortModuleDiagnostics(diagnostics)
	return result, diagnostics, nil
}

// ModulePath returns the root module path, or an empty string when disabled.
func (i ModuleIndex) ModulePath() string {
	return i.modulePath
}

// PackageFiles returns a copy of ordinary Go files in a package directory.
func (i ModuleIndex) PackageFiles(directory string) []string {
	directory = normalizePath(directory)
	files := i.packageFiles[directory]
	if len(files) == 0 {
		return nil
	}
	result := make([]string, len(files))
	copy(result, files)
	return result
}

// Owns reports whether path belongs to the supported root module.
func (i ModuleIndex) Owns(sourcePath string) bool {
	return i.modulePath != "" && strings.EqualFold(path.Ext(sourcePath), ".go") && !i.inNestedModule(sourcePath)
}

func (i ModuleIndex) inNestedModule(sourcePath string) bool {
	for _, nestedRoot := range i.nestedRoots {
		if inDirectory(sourcePath, nestedRoot) {
			return true
		}
	}
	return false
}

func normalizePaths(paths []string) []string {
	set := make(map[string]struct{}, len(paths))
	for _, candidate := range paths {
		normalized := normalizePath(candidate)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for candidate := range set {
		result = append(result, candidate)
	}
	sort.Strings(result)
	return result
}

func normalizePath(candidate string) string {
	normalized := path.Clean(strings.ReplaceAll(candidate, "\\", "/"))
	if normalized == "" || normalized == ".." || path.IsAbs(normalized) || strings.HasPrefix(normalized, "../") {
		return ""
	}
	return normalized
}

func inDirectory(candidate, directory string) bool {
	candidate = normalizePath(candidate)
	directory = normalizePath(directory)
	if candidate == "" || directory == "" || directory == "." {
		return false
	}
	return candidate == directory || strings.HasPrefix(candidate, directory+"/")
}

func sortModuleDiagnostics(diagnostics []ModuleDiagnostic) {
	sort.Slice(diagnostics, func(left, right int) bool {
		if diagnostics[left].Path == diagnostics[right].Path {
			return diagnostics[left].Message < diagnostics[right].Message
		}
		return diagnostics[left].Path < diagnostics[right].Path
	})
}
