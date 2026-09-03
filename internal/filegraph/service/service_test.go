package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
)

type fakeArchiveClient struct {
	archive    []byte
	err        error
	repository githubsource.Repository
	ref        string
	calls      int
}

func (f *fakeArchiveClient) FetchRepositoryZIP(_ context.Context, repository githubsource.Repository, ref string) ([]byte, error) {
	f.calls++
	f.repository = repository
	f.ref = ref
	return f.archive, f.err
}

func TestAnalyzeDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.tsx", `import "./Button"`)
	writeFile(t, root, "src/Button.tsx", "export const Button = true")

	service := New(nil, Options{WorkerCount: 2, ShutdownTimeout: time.Second})
	report, err := service.AnalyzeDirectory(context.Background(), root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if report.Root != "demo" || len(report.Nodes) != 2 || len(report.Edges) != 1 {
		t.Fatalf("report: %+v", report)
	}
}

func TestAnalyzeGitHubUsesRefAndCleansTemporaryDirectory(t *testing.T) {
	client := &fakeArchiveClient{archive: testArchive(t, map[string]string{
		"owner-repo-sha/app.py":    "from . import models\n",
		"owner-repo-sha/models.py": "class Model:\n    pass\n",
	})}
	temporaryParent := t.TempDir()
	service := New(client, Options{ShutdownTimeout: time.Second})
	service.temporaryParent = temporaryParent

	report, err := service.AnalyzeGitHub(context.Background(), GitHubRequest{
		RepositoryURL: "https://github.com/owner/repo",
		Ref:           "feature/files",
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.repository != (githubsource.Repository{Owner: "owner", Name: "repo"}) || client.ref != "feature/files" {
		t.Fatalf("request: repository=%+v ref=%q", client.repository, client.ref)
	}
	if report.Root != "repo" || len(report.Nodes) != 2 {
		t.Fatalf("report: %+v", report)
	}
	entries, err := os.ReadDir(temporaryParent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary entries remain: %+v", entries)
	}
}

func TestAnalyzeGitHubRejectsInvalidInputBeforeDownload(t *testing.T) {
	client := &fakeArchiveClient{}
	service := New(client, Options{})
	_, err := service.AnalyzeGitHub(context.Background(), GitHubRequest{RepositoryURL: "https://example.com/owner/repo"})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error: %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("archive calls: got %d, want 0", client.calls)
	}
}

func TestAnalyzeDirectoryHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app.ts", "export const app = true")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(nil, Options{ShutdownTimeout: time.Second}).AnalyzeDirectory(ctx, root, "demo")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error: got %v, want context canceled", err)
	}
}

func writeFile(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, contents := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
