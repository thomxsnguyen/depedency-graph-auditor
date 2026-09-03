package github

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchRepositoryZIP(t *testing.T) {
	archive := repositoryZIP(t, map[string]string{"owner-repo-sha/src/app.py": "import os\n"})
	tests := []struct {
		name     string
		ref      string
		wantPath string
	}{
		{name: "default branch", wantPath: "/repos/owner/repo/zipball"},
		{name: "explicit ref", ref: "feature/python", wantPath: "/repos/owner/repo/zipball/feature/python"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.wantPath {
					t.Errorf("path: got %q, want %q", request.URL.Path, test.wantPath)
				}
				if request.Header.Get("Accept") != "application/vnd.github+json" {
					t.Errorf("Accept: %q", request.Header.Get("Accept"))
				}
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(archive)
			}))
			defer server.Close()

			client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
			got, err := client.FetchRepositoryZIP(context.Background(), Repository{Owner: "owner", Name: "repo"}, test.ref)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, archive) {
				t.Fatal("downloaded archive differs")
			}
		})
	}
}

func TestFetchRepositoryZIPErrors(t *testing.T) {
	for _, test := range []struct {
		status int
		kind   error
	}{
		{status: http.StatusNotFound, kind: ErrRepositoryNotFound},
		{status: http.StatusForbidden, kind: ErrRateLimited},
		{status: http.StatusInternalServerError, kind: ErrUpstream},
	} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
			}))
			defer server.Close()
			client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
			if _, err := client.FetchRepositoryZIP(context.Background(), Repository{Owner: "owner", Name: "repo"}, ""); !errors.Is(err, test.kind) {
				t.Fatalf("status %d error: got %v, want kind %v", test.status, err, test.kind)
			}
		})
	}
}

func TestExtractRepositoryZIP(t *testing.T) {
	data := repositoryZIP(t, map[string]string{
		"owner-repo-sha/src/pc_diagnostic/__init__.py": "",
		"owner-repo-sha/src/pc_diagnostic/main.py":     "from pc_diagnostic import models\n",
	})
	destination := t.TempDir()
	root, err := ExtractRepositoryZIP(data, destination)
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Join(destination, "owner-repo-sha") {
		t.Fatalf("root: got %q", root)
	}
	contents, err := os.ReadFile(filepath.Join(root, "src", "pc_diagnostic", "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "from pc_diagnostic import models\n" {
		t.Fatalf("contents: %q", contents)
	}
}

func TestExtractRepositoryZIPRejectsUnsafeArchives(t *testing.T) {
	tests := []struct {
		name string
		zip  func(*testing.T) []byte
	}{
		{
			name: "path traversal",
			zip: func(t *testing.T) []byte {
				return repositoryZIP(t, map[string]string{"../escape.py": ""})
			},
		},
		{
			name: "multiple roots",
			zip: func(t *testing.T) []byte {
				return repositoryZIP(t, map[string]string{"root-a/app.py": "", "root-b/app.py": ""})
			},
		},
		{
			name: "symbolic link",
			zip:  symlinkRepositoryZIP,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ExtractRepositoryZIP(test.zip(t), t.TempDir()); err == nil {
				t.Fatal("expected unsafe archive error")
			}
		})
	}
}

func TestExtractRepositoryZIPRejectsInvalidAndEmptyArchives(t *testing.T) {
	if _, err := ExtractRepositoryZIP([]byte("not zip"), t.TempDir()); err == nil {
		t.Fatal("expected invalid ZIP error")
	}
	empty := repositoryZIP(t, nil)
	if _, err := ExtractRepositoryZIP(empty, t.TempDir()); err == nil {
		t.Fatal("expected empty ZIP error")
	}
}

func repositoryZIP(t *testing.T, files map[string]string) []byte {
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

func symlinkRepositoryZIP(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: "owner-repo-sha/link.py", Method: zip.Store}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("target.py")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestFetchRepositoryZIPRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = strings.NewReader(strings.Repeat("x", int(maxArchiveBytes)+1)).WriteTo(writer)
	}))
	defer server.Close()
	client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	if _, err := client.FetchRepositoryZIP(context.Background(), Repository{Owner: "owner", Name: "repo"}, ""); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("error: got %v, want archive-too-large kind", err)
	}
}
