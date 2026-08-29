package github_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
)

func TestParseRepositoryURL(t *testing.T) {
	tests := []struct {
		input   string
		want    githubsource.Repository
		wantErr bool
	}{
		{input: "https://github.com/acme/widget", want: githubsource.Repository{Owner: "acme", Name: "widget"}},
		{input: "https://github.com/acme/widget/", want: githubsource.Repository{Owner: "acme", Name: "widget"}},
		{input: "https://github.com/acme/widget.git", want: githubsource.Repository{Owner: "acme", Name: "widget"}},
		{input: "http://github.com/acme/widget", wantErr: true},
		{input: "https://example.com/acme/widget", wantErr: true},
		{input: "https://github.com/acme", wantErr: true},
		{input: "https://github.com/acme/widget/tree/main", wantErr: true},
		{input: "https://github.com//acme/widget", wantErr: true},
		{input: "https://github.com/acme/widget//", wantErr: true},
		{input: "https://user@github.com/acme/widget", wantErr: true},
		{input: "https://github.com/acme/widget?ref=main", wantErr: true},
		{input: "https://github.com/acme/widget#readme", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := githubsource.ParseRepositoryURL(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("repository=%+v error=%v, want %+v", got, err, test.want)
			}
		})
	}
}

func TestValidateManifestPath(t *testing.T) {
	for _, valid := range []string{"package.json", "packages/web/package.json"} {
		if err := githubsource.ValidateManifestPath(valid); err != nil {
			t.Errorf("valid path %q: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "/package.json", "./package.json", "packages/../package.json", "packages//package.json", "packages/"} {
		if err := githubsource.ValidateManifestPath(invalid); err == nil {
			t.Errorf("invalid path %q was accepted", invalid)
		}
	}
}

func TestFetchManifestRequest(t *testing.T) {
	const token = "secret-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method: got %s", request.Method)
		}
		if request.URL.Path != "/repos/acme/widget/contents/packages/web app/package.json" {
			t.Errorf("path: got %q", request.URL.Path)
		}
		if !strings.Contains(request.RequestURI, "web%20app") {
			t.Errorf("manifest path was not URL-escaped: %q", request.RequestURI)
		}
		if request.URL.Query().Get("ref") != "feature/a b" {
			t.Errorf("ref: got %q", request.URL.Query().Get("ref"))
		}
		if request.Header.Get("Accept") != "application/vnd.github.raw+json" {
			t.Errorf("accept: got %q", request.Header.Get("Accept"))
		}
		if request.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("authorization header missing")
		}
		_, _ = io.WriteString(writer, `{"name":"widget"}`)
	}))
	defer server.Close()

	client := githubsource.GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL, Token: token}
	data, err := client.FetchManifest(context.Background(), githubsource.Repository{Owner: "acme", Name: "widget"}, "packages/web app/package.json", "feature/a b")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"widget"}` {
		t.Fatalf("body: got %q", data)
	}
}

func TestFetchManifestDefaultBranchIsUnauthenticated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" {
			t.Errorf("query: got %q, want empty", request.URL.RawQuery)
		}
		if request.Header.Get("Authorization") != "" {
			t.Errorf("authorization: got %q, want empty", request.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()

	client := githubsource.GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	if _, err := client.FetchManifest(context.Background(), githubsource.Repository{Owner: "acme", Name: "widget"}, "package.json", ""); err != nil {
		t.Fatal(err)
	}
}

func TestFetchManifestErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(status)
			}))
			defer server.Close()

			client := githubsource.GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL, Token: "never-print-this"}
			_, err := client.FetchManifest(context.Background(), githubsource.Repository{Owner: "acme", Name: "widget"}, "package.json", "")
			if err == nil || strings.Contains(err.Error(), client.Token) {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestFetchManifestRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, strings.Repeat("x", (1<<20)+1))
	}))
	defer server.Close()

	client := githubsource.GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	_, err := client.FetchManifest(context.Background(), githubsource.Repository{Owner: "acme", Name: "widget"}, "package.json", "")
	if err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("error: got %v, want size limit", err)
	}
}

func TestFetchManifestHonorsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	client := githubsource.GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	_, err := client.FetchManifest(ctx, githubsource.Repository{Owner: "acme", Name: "widget"}, "package.json", "")
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("error: got %v, want deadline", err)
	}
}
