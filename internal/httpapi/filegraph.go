// Package httpapi exposes the bounded HTTP surface used by the local demo UI.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	filegraphservice "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/service"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
)

const maxFileGraphRequestBytes = 4 << 10

type FileGraphAnalyzer interface {
	AnalyzeGitHub(context.Context, filegraphservice.GitHubRequest) (filegraph.Report, error)
}

type FileGraphHandler struct {
	analyzer FileGraphAnalyzer
	logger   *log.Logger
}

type fileGraphRequest struct {
	RepositoryURL string  `json:"repositoryUrl"`
	Ref           *string `json:"ref,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewFileGraphHandler(analyzer FileGraphAnalyzer, logger *log.Logger) *FileGraphHandler {
	return &FileGraphHandler{analyzer: analyzer, logger: logger}
}

func (h *FileGraphHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, "Use POST to analyze a repository.")
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(writer, http.StatusUnsupportedMediaType, "Use an application/json request body.")
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, maxFileGraphRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input fileGraphRequest
	if err := decoder.Decode(&input); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(writer, http.StatusRequestEntityTooLarge, "The request body is too large.")
			return
		}
		writeError(writer, http.StatusBadRequest, "Enter a valid JSON request.")
		return
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(writer, http.StatusRequestEntityTooLarge, "The request body is too large.")
			return
		}
		writeError(writer, http.StatusBadRequest, "Enter a single JSON request object.")
		return
	}

	repositoryURL := strings.TrimSpace(input.RepositoryURL)
	if _, err := githubsource.ParseRepositoryURL(repositoryURL); err != nil {
		writeError(writer, http.StatusBadRequest, "Use https://github.com/owner/repository.")
		return
	}
	ref := ""
	if input.Ref != nil {
		ref = strings.TrimSpace(*input.Ref)
		if ref == "" {
			writeError(writer, http.StatusBadRequest, "Enter a branch, tag, or commit, or omit ref.")
			return
		}
	}
	if h.analyzer == nil {
		writeError(writer, http.StatusInternalServerError, "The GitHub repository could not be analyzed.")
		return
	}

	report, err := h.analyzer.AnalyzeGitHub(request.Context(), filegraphservice.GitHubRequest{
		RepositoryURL: repositoryURL,
		Ref:           ref,
	})
	if err != nil {
		if request.Context().Err() != nil {
			return
		}
		status, message := classifyFileGraphError(err)
		if h.logger != nil {
			h.logger.Printf("file graph request failed: category=%s", http.StatusText(status))
		}
		writeError(writer, status, message)
		return
	}

	writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(writer).Encode(report)
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("unexpected trailing JSON")
	}
	return err
}

func classifyFileGraphError(err error) (int, string) {
	switch {
	case errors.Is(err, filegraphservice.ErrInvalidRequest):
		return http.StatusBadRequest, "Use https://github.com/owner/repository."
	case errors.Is(err, githubsource.ErrRepositoryNotFound):
		return http.StatusNotFound, "The GitHub repository or ref was not found."
	case errors.Is(err, githubsource.ErrArchiveTooLarge):
		return http.StatusRequestEntityTooLarge, "The GitHub repository is too large to analyze."
	case errors.Is(err, githubsource.ErrRateLimited):
		return http.StatusTooManyRequests, "GitHub rate limited the request. Try again later."
	case errors.Is(err, githubsource.ErrUpstream):
		return http.StatusBadGateway, "GitHub could not be reached. Try again later."
	default:
		return http.StatusInternalServerError, "The GitHub repository could not be analyzed."
	}
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(errorResponse{Error: message})
}
