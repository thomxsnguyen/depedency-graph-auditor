package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/pypi"
)

type DependencyAuditPayload struct {
	RepositoryURL string `json:"repositoryUrl"`
	Ref           string `json:"ref,omitempty"`
}
type ManifestFetcher interface {
	FetchManifest(context.Context, githubsource.Repository, string, string) ([]byte, error)
}
type DependencyAuditHandler struct{ GitHub ManifestFetcher }

func (h DependencyAuditHandler) Handle(ctx context.Context, value job.Job) (job.HandlerResult, error) {
	var payload DependencyAuditPayload
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, fmt.Errorf("invalid dependency audit payload: %w", err))
	}
	repository, err := githubsource.ParseRepositoryURL(strings.TrimSpace(payload.RepositoryURL))
	if err != nil {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, err)
	}
	target, _ := pypi.NewTarget("3.12", "linux")
	type manifestSpec struct {
		path      string
		ecosystem string
		jobType   string
		parse     func([]byte) (depfile.Manifest, error)
	}
	specs := []manifestSpec{
		{path: "package.json", ecosystem: "npm", jobType: "audit_npm_package", parse: func(data []byte) (depfile.Manifest, error) {
			return depfile.ParsePackageJSON(bytes.NewReader(data), true)
		}},
		{path: "pyproject.toml", ecosystem: "pypi", jobType: "audit_pypi_package", parse: func(data []byte) (depfile.Manifest, error) {
			return depfile.ParsePyProject(bytes.NewReader(data), target)
		}},
		{path: "requirements.txt", ecosystem: "pypi", jobType: "audit_pypi_package", parse: func(data []byte) (depfile.Manifest, error) {
			return depfile.ParseRequirements(bytes.NewReader(data), repository.Name, target)
		}},
		{path: "go.mod", ecosystem: "go", jobType: "audit_go_module", parse: func(data []byte) (depfile.Manifest, error) {
			manifest, err := depfile.ParseGoMod(bytes.NewReader(data))
			return manifest.Manifest, err
		}},
	}
	children := []job.Submission{}
	roots := []string{}
	for _, spec := range specs {
		manifestData, err := h.GitHub.FetchManifest(ctx, repository, spec.path, strings.TrimSpace(payload.Ref))
		if err != nil {
			if strings.Contains(err.Error(), "was not found") {
				continue
			}
			return job.HandlerResult{}, classifyError(err)
		}
		manifest, err := spec.parse(manifestData)
		if err != nil {
			return job.HandlerResult{}, job.Failure(job.ErrorPermanent, err)
		}
		roots = append(roots, spec.ecosystem+":"+manifest.Name)
		for _, dependency := range manifest.Dependencies {
			body, err := json.Marshal(auditor.AuditPayload{Name: dependency.Name, Version: dependency.VersionRange, ParentName: manifest.Name})
			if err != nil {
				return job.HandlerResult{}, job.Failure(job.ErrorPermanent, err)
			}
			sum := sha256.Sum256([]byte(value.RootJobID + "\x00" + spec.ecosystem + "\x00" + dependency.Name + "\x00" + dependency.VersionRange))
			children = append(children, job.Submission{Type: spec.jobType, Payload: body, MaxAttempts: value.MaxAttempts, RootJobID: value.RootJobID, ParentJobID: value.ID, Internal: true, IdempotencyKey: "audit-child:" + hex.EncodeToString(sum[:])})
		}
	}
	if len(roots) == 0 {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, fmt.Errorf("repository contains no supported dependency manifest"))
	}
	result, _ := json.Marshal(map[string]any{"auditId": value.RootJobID, "repositoryUrl": payload.RepositoryURL, "roots": roots, "seedCount": len(children)})
	return job.HandlerResult{Result: result, Children: children}, nil
}

func classifyError(err error) error {
	var networkError net.Error
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "rate limit") || strings.Contains(message, "status 429") || strings.Contains(message, "status 502") || strings.Contains(message, "status 503") || strings.Contains(message, "status 504") || errors.As(err, &networkError) {
		return job.Failure(job.ErrorTransient, err)
	}
	return job.Failure(job.ErrorPermanent, err)
}
