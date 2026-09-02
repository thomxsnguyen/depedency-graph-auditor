package filegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// JobType identifies one file dependency analysis job.
const JobType = "analyze_file"

// Payload identifies one source file beneath a validated project root.
type Payload struct {
	Root string `json:"root"`
	Path string `json:"path"`
}

// Handler extracts and records local imports for one source file.
type Handler struct {
	root     string
	index    RepositoryIndex
	registry *analyzer.Registry
	store    *Store
}

// NewHandler constructs a file graph job handler.
func NewHandler(root string, index RepositoryIndex, registry *analyzer.Registry, store *Store) (*Handler, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("filegraph: resolve handler root %q: %w", root, err)
	}
	if store == nil {
		return nil, fmt.Errorf("filegraph: store is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("filegraph: analyzer registry is required")
	}
	return &Handler{
		root:     filepath.Clean(absoluteRoot),
		index:    index,
		registry: registry,
		store:    store,
	}, nil
}

// Handle implements job.Handler for analyze_file jobs.
func (h *Handler) Handle(ctx context.Context, queued job.Job) ([]job.Job, error) {
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
	_, relativePath, err := safeSourcePath(h.root, payload.Path)
	if err != nil {
		return nil, err
	}
	if !h.index.Has(relativePath) {
		return nil, fmt.Errorf("filegraph: source path %q is not in the project index", relativePath)
	}

	selected, supported, err := h.registry.AnalyzerFor(relativePath)
	if err != nil {
		return nil, err
	}
	if !supported {
		h.store.AddDiagnostic(Diagnostic{Path: relativePath, Message: "unsupported source language"})
		return nil, nil
	}

	result, err := selected.Analyze(ctx, analyzer.FileContext{
		Root:  h.root,
		Path:  relativePath,
		Index: h.index,
	})
	if err != nil {
		return nil, fmt.Errorf("filegraph: analyze %q: %w", relativePath, err)
	}
	for _, dependency := range result.Dependencies {
		h.store.AddEdge(Edge{From: relativePath, To: dependency.Target})
	}
	for _, diagnostic := range result.Diagnostics {
		h.store.AddDiagnostic(Diagnostic{
			Path:    relativePath,
			Import:  diagnostic.Reference,
			Message: diagnostic.Message,
		})
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
