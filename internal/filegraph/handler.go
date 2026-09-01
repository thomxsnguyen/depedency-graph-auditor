package filegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

const (
	// JobType identifies one file dependency analysis job.
	JobType            = "analyze_file"
	maxSourceFileBytes = int64(1 << 20)
)

// Payload identifies one source file beneath a validated project root.
type Payload struct {
	Root string `json:"root"`
	Path string `json:"path"`
}

type sourceReader func(string) ([]byte, error)

// Handler extracts and records local imports for one source file.
type Handler struct {
	root       string
	index      Index
	store      *Store
	readSource sourceReader
}

// NewHandler constructs a file graph job handler.
func NewHandler(root string, index Index, store *Store) (*Handler, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("filegraph: resolve handler root %q: %w", root, err)
	}
	if store == nil {
		return nil, fmt.Errorf("filegraph: store is required")
	}
	return &Handler{
		root:       filepath.Clean(absoluteRoot),
		index:      index,
		store:      store,
		readSource: readSourceFile,
	}, nil
}

// Handle implements job.Handler for analyze_file jobs.
func (h *Handler) Handle(_ context.Context, queued job.Job) ([]job.Job, error) {
	if queued.Type != JobType {
		return nil, fmt.Errorf("filegraph: unsupported job type %q", queued.Type)
	}

	var payload Payload
	if err := json.Unmarshal(queued.Payload, &payload); err != nil {
		return nil, fmt.Errorf("filegraph: decode job %s payload: %w", queued.ID, err)
	}
	if filepath.Clean(payload.Root) != h.root {
		return nil, fmt.Errorf("filegraph: job root does not match configured project root")
	}
	absolutePath, relativePath, err := safeSourcePath(h.root, payload.Path)
	if err != nil {
		return nil, err
	}
	if _, exists := h.index[relativePath]; !exists {
		return nil, fmt.Errorf("filegraph: source path %q is not in the project index", relativePath)
	}

	source, err := h.readSource(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("filegraph: read %q: %w", relativePath, err)
	}
	imports, err := ExtractImports(source)
	if err != nil {
		h.store.AddDiagnostic(Diagnostic{Path: relativePath, Message: err.Error()})
		return nil, nil
	}
	for _, specifier := range imports {
		resolved, ok := Resolve(h.index, relativePath, specifier)
		if !ok {
			h.store.AddDiagnostic(Diagnostic{
				Path:    relativePath,
				Import:  specifier,
				Message: "unresolved local import",
			})
			continue
		}
		h.store.AddEdge(Edge{From: relativePath, To: resolved})
	}
	return nil, nil
}

// NewJob constructs one analyze_file job.
func NewJob(root, path string) (job.Job, error) {
	payload, err := json.Marshal(Payload{Root: root, Path: path})
	if err != nil {
		return job.Job{}, fmt.Errorf("filegraph: encode job for %q: %w", path, err)
	}
	return job.NewJob(JobType, payload), nil
}

func safeSourcePath(root, relative string) (string, string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", "", fmt.Errorf("filegraph: source path must be project-relative")
	}
	normalized := filepath.Clean(filepath.FromSlash(relative))
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("filegraph: source path %q escapes the project root", relative)
	}
	absolute := filepath.Join(root, normalized)
	relativeToRoot, err := filepath.Rel(root, absolute)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("filegraph: source path %q escapes the project root", relative)
	}
	return absolute, filepath.ToSlash(normalized), nil
}

func readSourceFile(path string) ([]byte, error) {
	file, err := os.Open(path)
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
