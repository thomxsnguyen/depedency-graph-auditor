package handlers

import (
	"context"
	"fmt"
	"testing"

	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

type manifestFetcherStub map[string]string

func (f manifestFetcherStub) FetchManifest(_ context.Context, _ githubsource.Repository, path, _ string) ([]byte, error) {
	if value, exists := f[path]; exists {
		return []byte(value), nil
	}
	return nil, fmt.Errorf("GitHub manifest %q was not found", path)
}

func TestDependencyAuditSeedsOnlyExistingEcosystems(t *testing.T) {
	handler := DependencyAuditHandler{GitHub: manifestFetcherStub{
		"package.json": `{"name":"web","dependencies":{"react":"^19.0.0"}}`,
		"go.mod":       "module example.com/service\n\ngo 1.24\n\nrequire example.com/lib v1.2.3\n",
	}}
	result, err := handler.Handle(context.Background(), job.Job{ID: "root", RootJobID: "root",
		Type: "dependency_audit", Payload: []byte(`{"repositoryUrl":"https://github.com/acme/service"}`), MaxAttempts: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Children) != 2 {
		t.Fatalf("children=%d", len(result.Children))
	}
	seen := map[string]bool{}
	for _, child := range result.Children {
		seen[child.Type] = true
		if !child.Internal || child.IdempotencyKey == "" || child.RootJobID != "root" {
			t.Fatalf("child=%+v", child)
		}
	}
	if !seen["audit_npm_package"] || !seen["audit_go_module"] || seen["audit_pypi_package"] {
		t.Fatalf("types=%v", seen)
	}
}

func TestDependencyAuditRejectsRepositoryWithoutManifest(t *testing.T) {
	handler := DependencyAuditHandler{GitHub: manifestFetcherStub{}}
	_, err := handler.Handle(context.Background(), job.Job{ID: "root", RootJobID: "root",
		Payload: []byte(`{"repositoryUrl":"https://github.com/acme/empty"}`)})
	if err == nil || job.KindOf(err) != job.ErrorPermanent {
		t.Fatalf("error=%v kind=%s", err, job.KindOf(err))
	}
}
