package analyzer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxSourceFileBytes = int64(1 << 20)

// ReadSource reads one validated repository-relative source file with a limit.
func ReadSource(root, relative string) ([]byte, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	absoluteRoot = filepath.Clean(absoluteRoot)
	if relative == "" || filepath.IsAbs(relative) {
		return nil, fmt.Errorf("source path must be repository-relative")
	}
	normalized := filepath.Clean(filepath.FromSlash(relative))
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("source path %q escapes the repository root", relative)
	}
	absolutePath := filepath.Join(absoluteRoot, normalized)
	relativeToRoot, err := filepath.Rel(absoluteRoot, absolutePath)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("source path %q escapes the repository root", relative)
	}

	file, err := os.Open(absolutePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxSourceFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSourceFileBytes {
		return nil, fmt.Errorf("source file exceeds the 1 MiB limit")
	}
	return data, nil
}
