package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	filegraphservice "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/service"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
)

type fakeFileGraphAnalyzer struct {
	report  filegraph.Report
	err     error
	request filegraphservice.GitHubRequest
	calls   int
}

func (f *fakeFileGraphAnalyzer) AnalyzeGitHub(_ context.Context, request filegraphservice.GitHubRequest) (filegraph.Report, error) {
	f.calls++
	f.request = request
	return f.report, f.err
}

func TestFileGraphHandlerReturnsReport(t *testing.T) {
	analyzer := &fakeFileGraphAnalyzer{report: filegraph.Report{
		SchemaVersion: 1,
		Root:          "repo",
		Nodes:         []filegraph.Node{{Path: "app.py"}},
		Edges:         []filegraph.Edge{},
		Diagnostics:   []filegraph.Diagnostic{},
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/file-graphs", strings.NewReader(
		`{"repositoryUrl":"https://github.com/owner/repo","ref":"main"}`,
	))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	NewFileGraphHandler(analyzer, nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if analyzer.request.RepositoryURL != "https://github.com/owner/repo" || analyzer.request.Ref != "main" {
		t.Fatalf("request: %+v", analyzer.request)
	}
	if !strings.Contains(recorder.Body.String(), `"schemaVersion":1`) || !strings.Contains(recorder.Body.String(), `"nodes":[{"path":"app.py"}]`) {
		t.Fatalf("body: %s", recorder.Body.String())
	}
}

func TestFileGraphHandlerRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "method", method: http.MethodGet, contentType: "application/json", body: `{}`, wantStatus: http.StatusMethodNotAllowed},
		{name: "content type", method: http.MethodPost, contentType: "text/plain", body: `{}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "unknown field", method: http.MethodPost, contentType: "application/json", body: `{"repositoryUrl":"https://github.com/o/r","extra":true}`, wantStatus: http.StatusBadRequest},
		{name: "invalid URL", method: http.MethodPost, contentType: "application/json", body: `{"repositoryUrl":"https://example.com/o/r"}`, wantStatus: http.StatusBadRequest},
		{name: "blank ref", method: http.MethodPost, contentType: "application/json", body: `{"repositoryUrl":"https://github.com/o/r","ref":" "}`, wantStatus: http.StatusBadRequest},
		{name: "trailing JSON", method: http.MethodPost, contentType: "application/json", body: `{"repositoryUrl":"https://github.com/o/r"} {}`, wantStatus: http.StatusBadRequest},
		{name: "body too large", method: http.MethodPost, contentType: "application/json", body: strings.Repeat(" ", maxFileGraphRequestBytes+1), wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer := &fakeFileGraphAnalyzer{}
			request := httptest.NewRequest(test.method, "/api/file-graphs", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()
			NewFileGraphHandler(analyzer, nil).ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status: got %d, want %d body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if analyzer.calls != 0 {
				t.Fatalf("analyzer calls: got %d, want 0", analyzer.calls)
			}
		})
	}
}

func TestFileGraphHandlerMapsSafeErrors(t *testing.T) {
	tests := []struct {
		err        error
		wantStatus int
	}{
		{err: filegraphservice.ErrInvalidRequest, wantStatus: http.StatusBadRequest},
		{err: githubsource.ErrRepositoryNotFound, wantStatus: http.StatusNotFound},
		{err: githubsource.ErrArchiveTooLarge, wantStatus: http.StatusRequestEntityTooLarge},
		{err: githubsource.ErrRateLimited, wantStatus: http.StatusTooManyRequests},
		{err: githubsource.ErrUpstream, wantStatus: http.StatusBadGateway},
		{err: errors.New("temporary path /private/tmp/secret"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		analyzer := &fakeFileGraphAnalyzer{err: test.err}
		request := httptest.NewRequest(http.MethodPost, "/api/file-graphs", strings.NewReader(
			`{"repositoryUrl":"https://github.com/owner/repo"}`,
		))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		NewFileGraphHandler(analyzer, nil).ServeHTTP(recorder, request)
		if recorder.Code != test.wantStatus {
			t.Fatalf("error %v: status got %d, want %d", test.err, recorder.Code, test.wantStatus)
		}
		if strings.Contains(recorder.Body.String(), "/private/tmp") {
			t.Fatalf("unsafe body: %s", recorder.Body.String())
		}
	}
}
