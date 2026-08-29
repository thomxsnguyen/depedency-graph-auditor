// Package github retrieves package manifests from public GitHub repositories.
package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL      = "https://api.github.com"
	defaultMaxBodyBytes = int64(1 << 20)
	defaultTimeout      = 15 * time.Second
)

// Repository identifies a GitHub repository.
type Repository struct {
	Owner string
	Name  string
}

// GitHubClient retrieves raw manifest bytes through the GitHub Contents API.
type GitHubClient struct {
	HTTPClient *http.Client
	BaseURL    string
	Token      string
}

// ParseRepositoryURL validates and normalizes a public github.com repository URL.
func ParseRepositoryURL(raw string) (Repository, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return Repository{}, fmt.Errorf("invalid GitHub repository URL %q: %w", raw, err)
	}
	if parsed.Scheme != "https" {
		return Repository{}, fmt.Errorf("GitHub repository URL must use https")
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.Port() != "" {
		return Repository{}, fmt.Errorf("GitHub repository URL host must be github.com")
	}
	if parsed.User != nil {
		return Repository{}, fmt.Errorf("GitHub repository URL must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return Repository{}, fmt.Errorf("GitHub repository URL must not contain a query or fragment")
	}

	repositoryPath := strings.TrimPrefix(parsed.Path, "/")
	repositoryPath = strings.TrimSuffix(repositoryPath, "/")
	segments := strings.Split(repositoryPath, "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return Repository{}, fmt.Errorf("GitHub repository URL must have the form https://github.com/owner/repository")
	}
	repositoryName := strings.TrimSuffix(segments[1], ".git")
	if repositoryName == "" {
		return Repository{}, fmt.Errorf("GitHub repository URL must include a repository name")
	}
	return Repository{Owner: segments[0], Name: repositoryName}, nil
}

// ValidateManifestPath validates a repository-relative POSIX file path.
func ValidateManifestPath(manifestPath string) error {
	if manifestPath == "" {
		return fmt.Errorf("manifest path must be non-empty")
	}
	if strings.HasPrefix(manifestPath, "/") {
		return fmt.Errorf("manifest path must be repository-relative")
	}
	segments := strings.Split(manifestPath, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("manifest path contains invalid segment %q", segment)
		}
	}
	return nil
}

// FetchManifest retrieves one raw manifest from a repository.
func (c *GitHubClient) FetchManifest(ctx context.Context, repository Repository, manifestPath, ref string) ([]byte, error) {
	if err := ValidateManifestPath(manifestPath); err != nil {
		return nil, err
	}
	if repository.Owner == "" || repository.Name == "" {
		return nil, fmt.Errorf("GitHub repository owner and name are required")
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub API base URL: %w", err)
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/repos/" +
		repository.Owner + "/" + repository.Name + "/contents/" + manifestPath
	if ref != "" {
		query := endpoint.Query()
		query.Set("ref", ref)
		endpoint.RawQuery = query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create GitHub manifest request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github.raw+json")
	request.Header.Set("User-Agent", "mini-distributed-job-api")
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub manifest: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("GitHub manifest %q was not found in %s/%s", manifestPath, repository.Owner, repository.Name)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, fmt.Errorf("GitHub API rate limit or access restriction for %s/%s manifest %q (status %d)", repository.Owner, repository.Name, manifestPath, response.StatusCode)
	default:
		return nil, fmt.Errorf("GitHub API returned status %d for %s/%s manifest %q", response.StatusCode, repository.Owner, repository.Name, manifestPath)
	}

	limited := io.LimitReader(response.Body, defaultMaxBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read GitHub manifest response: %w", err)
	}
	if int64(len(data)) > defaultMaxBodyBytes {
		return nil, fmt.Errorf("GitHub manifest exceeds the 1 MiB limit")
	}
	return data, nil
}
