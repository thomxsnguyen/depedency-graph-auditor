package javascript

import (
	"path"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
)

var resolutionExtensions = []string{".ts", ".tsx", ".js", ".jsx"}

// Resolve maps one relative import specifier to an indexed source file.
func Resolve(index analyzer.Index, importer, specifier string) (string, bool) {
	if !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") {
		return "", false
	}

	base := path.Clean(path.Join(path.Dir(importer), specifier))
	if base == ".." || strings.HasPrefix(base, "../") || path.IsAbs(base) {
		return "", false
	}

	candidates := make([]string, 0, 9)
	if supportedExtension(path.Ext(base)) {
		candidates = append(candidates, base)
	} else {
		for _, extension := range resolutionExtensions {
			candidates = append(candidates, base+extension)
		}
		for _, extension := range resolutionExtensions {
			candidates = append(candidates, path.Join(base, "index"+extension))
		}
	}

	for _, candidate := range candidates {
		if index.Has(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func supportedExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".js", ".jsx", ".ts", ".tsx":
		return true
	default:
		return false
	}
}
