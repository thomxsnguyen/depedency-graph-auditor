// Package service orchestrates complete file dependency graph analysis.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	fileanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	goanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/golang"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/javascript"
	pythonanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/python"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

const (
	defaultWorkerCount     = 10
	defaultShutdownTimeout = 30 * time.Second
)

var ErrInvalidRequest = errors.New("invalid GitHub file graph request")

type GitHubRequest struct {
	RepositoryURL string
	Ref           string
}

type Options struct {
	WorkerCount     int
	ShutdownTimeout time.Duration
}

type ArchiveClient interface {
	FetchRepositoryZIP(context.Context, githubsource.Repository, string) ([]byte, error)
}

type Service struct {
	archiveClient   ArchiveClient
	workerCount     int
	shutdownTimeout time.Duration
	temporaryParent string
}

func New(archiveClient ArchiveClient, options Options) *Service {
	if archiveClient == nil {
		archiveClient = &githubsource.GitHubClient{}
	}
	workerCount := options.WorkerCount
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
	}
	shutdownTimeout := options.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}
	return &Service{
		archiveClient:   archiveClient,
		workerCount:     workerCount,
		shutdownTimeout: shutdownTimeout,
	}
}

func (s *Service) AnalyzeGitHub(ctx context.Context, request GitHubRequest) (filegraph.Report, error) {
	if err := ctx.Err(); err != nil {
		return filegraph.Report{}, err
	}
	repositoryURL := strings.TrimSpace(request.RepositoryURL)
	if repositoryURL == "" {
		return filegraph.Report{}, fmt.Errorf("%w: repository URL is required", ErrInvalidRequest)
	}
	repository, err := githubsource.ParseRepositoryURL(repositoryURL)
	if err != nil {
		return filegraph.Report{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if request.Ref != "" && strings.TrimSpace(request.Ref) == "" {
		return filegraph.Report{}, fmt.Errorf("%w: ref must not be blank", ErrInvalidRequest)
	}

	temporaryDirectory, err := os.MkdirTemp(s.temporaryParent, "auditor-github-*")
	if err != nil {
		return filegraph.Report{}, fmt.Errorf("create temporary GitHub repository directory: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)

	archive, err := s.archiveClient.FetchRepositoryZIP(ctx, repository, strings.TrimSpace(request.Ref))
	if err != nil {
		return filegraph.Report{}, err
	}
	repositoryRoot, err := githubsource.ExtractRepositoryZIP(archive, temporaryDirectory)
	if err != nil {
		return filegraph.Report{}, err
	}
	return s.AnalyzeDirectory(ctx, repositoryRoot, repository.Name)
}

func (s *Service) AnalyzeDirectory(ctx context.Context, root, reportRoot string) (filegraph.Report, error) {
	return s.AnalyzeDirectoryUntil(ctx, ctx, root, reportRoot)
}

// AnalyzeDirectoryUntil lets the CLI keep active handler contexts independent
// from its signal context while sharing the same analysis implementation.
func (s *Service) AnalyzeDirectoryUntil(workCtx, stopCtx context.Context, root, reportRoot string) (filegraph.Report, error) {
	if err := stopCtx.Err(); err != nil {
		return filegraph.Report{}, err
	}
	if err := workCtx.Err(); err != nil {
		return filegraph.Report{}, err
	}
	discovery, err := filegraph.DiscoverRepository(root)
	if err != nil {
		return filegraph.Report{}, err
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return filegraph.Report{}, fmt.Errorf("filegraph: resolve project root %q: %w", root, err)
	}
	absoluteRoot = filepath.Clean(absoluteRoot)
	moduleIndex, moduleDiagnostics, err := goanalyzer.BuildModuleIndex(absoluteRoot, discovery.Index, discovery.GoModules)
	if err != nil {
		return filegraph.Report{}, err
	}

	graphStore := filegraph.NewStore()
	for _, path := range discovery.Paths {
		graphStore.AddNode(filegraph.Node{Path: path})
	}
	for _, diagnostic := range moduleDiagnostics {
		graphStore.AddDiagnostic(filegraph.Diagnostic{Path: diagnostic.Path, Message: diagnostic.Message})
	}
	if len(discovery.Paths) > 0 {
		registry, err := fileanalyzer.NewRegistry(javascript.New(), pythonanalyzer.New(), goanalyzer.New(moduleIndex))
		if err != nil {
			return filegraph.Report{}, err
		}
		handler, err := filegraph.NewHandler(absoluteRoot, discovery.Index, registry, graphStore)
		if err != nil {
			return filegraph.Report{}, err
		}
		bufferSize := 100
		if len(discovery.Paths) > bufferSize {
			bufferSize = len(discovery.Paths)
		}
		q := queue.New(bufferSize)
		pool := worker.New(s.workerCount, q, handler)
		for _, path := range discovery.Paths {
			queued, err := filegraph.NewJob(absoluteRoot, path)
			if err != nil {
				return filegraph.Report{}, err
			}
			if err := pool.Submit(queued); err != nil {
				return filegraph.Report{}, fmt.Errorf("filegraph: submit %q: %w", path, err)
			}
		}
		pool.Start(workCtx)

		select {
		case <-pool.Done():
		case <-stopCtx.Done():
		}

		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancelShutdown()
		if err := pool.Shutdown(shutdownCtx); err != nil {
			return filegraph.Report{}, fmt.Errorf("filegraph: shutdown: %w", err)
		}
		if err := stopCtx.Err(); err != nil {
			return filegraph.Report{}, err
		}
	}

	if reportRoot == "" {
		reportRoot = filepath.Base(absoluteRoot)
	}
	if err := stopCtx.Err(); err != nil {
		return filegraph.Report{}, err
	}
	return filegraph.GenerateReport(reportRoot, graphStore), nil
}
