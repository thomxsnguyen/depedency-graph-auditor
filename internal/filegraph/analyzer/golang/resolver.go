package golang

import (
	"path"
	"strings"
)

// Resolution is the repository-local result for one Go package import.
type Resolution struct {
	Targets []string
	Local   bool
}

// Resolve maps a root-module import to all ordinary files in that package.
func Resolve(index ModuleIndex, importPath string) Resolution {
	modulePath := index.ModulePath()
	if modulePath == "" {
		return Resolution{}
	}

	directory := ""
	switch {
	case importPath == modulePath:
		directory = "."
	case strings.HasPrefix(importPath, modulePath+"/"):
		directory = strings.TrimPrefix(importPath, modulePath+"/")
	default:
		return Resolution{}
	}
	directory = path.Clean(directory)
	if directory == ".." || path.IsAbs(directory) || strings.HasPrefix(directory, "../") {
		return Resolution{Local: true}
	}
	return Resolution{Targets: index.PackageFiles(directory), Local: true}
}
