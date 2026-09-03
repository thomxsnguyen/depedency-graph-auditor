package github

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	// ErrRepositoryNotFound classifies a missing public repository or ref.
	ErrRepositoryNotFound = errors.New("GitHub repository or ref was not found")
	// ErrRateLimited classifies GitHub access restrictions and rate limits.
	ErrRateLimited = errors.New("GitHub API rate limit or access restriction")
	// ErrArchiveTooLarge classifies compressed or extracted archive limits.
	ErrArchiveTooLarge = errors.New("GitHub repository archive exceeds its size limit")
	// ErrUpstream classifies GitHub transport and unexpected response failures.
	ErrUpstream = errors.New("GitHub API request failed")
)

const (
	maxArchiveBytes        = int64(25 << 20)
	maxExtractedFileBytes  = int64(20 << 20)
	maxExtractedTotalBytes = int64(100 << 20)
	maxArchiveEntries      = 20_000
)

// FetchRepositoryZIP downloads one public repository archive. GitHub's
// redirect is followed by the configured HTTP client.
func (c *GitHubClient) FetchRepositoryZIP(ctx context.Context, repository Repository, ref string) ([]byte, error) {
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
		repository.Owner + "/" + repository.Name + "/zipball"
	if ref != "" {
		basePath := endpoint.EscapedPath()
		endpoint.Path += "/" + ref
		endpoint.RawPath = basePath + "/" + url.PathEscape(ref)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create GitHub repository archive request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
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
		return nil, fmt.Errorf("%w: fetch repository archive for %s/%s: %v", ErrUpstream, repository.Owner, repository.Name, err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s/%s ref %q", ErrRepositoryNotFound, repository.Owner, repository.Name, ref)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w for %s/%s archive (status %d)", ErrRateLimited, repository.Owner, repository.Name, response.StatusCode)
	default:
		return nil, fmt.Errorf("%w: status %d for %s/%s archive", ErrUpstream, response.StatusCode, repository.Owner, repository.Name)
	}

	limited := io.LimitReader(response.Body, maxArchiveBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("%w: read repository archive for %s/%s: %v", ErrUpstream, repository.Owner, repository.Name, err)
	}
	if int64(len(data)) > maxArchiveBytes {
		return nil, fmt.Errorf("%w: %s/%s exceeds the 25 MiB compressed limit", ErrArchiveTooLarge, repository.Owner, repository.Name)
	}
	return data, nil
}

// ExtractRepositoryZIP securely extracts one GitHub archive beneath
// destination and returns its single generated repository root.
func ExtractRepositoryZIP(data []byte, destination string) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("GitHub repository archive is not a valid ZIP: %w", err)
	}
	if len(archive.File) == 0 {
		return "", fmt.Errorf("GitHub repository archive is empty")
	}
	if len(archive.File) > maxArchiveEntries {
		return "", fmt.Errorf("%w: exceeds the %d-entry limit", ErrArchiveTooLarge, maxArchiveEntries)
	}

	absoluteDestination, err := filepath.Abs(destination)
	if err != nil {
		return "", fmt.Errorf("resolve archive destination %q: %w", destination, err)
	}
	if err := os.MkdirAll(absoluteDestination, 0o755); err != nil {
		return "", fmt.Errorf("create archive destination %q: %w", destination, err)
	}

	topLevel := ""
	var totalBytes uint64
	for _, entry := range archive.File {
		cleanName, root, err := validateArchiveEntry(entry)
		if err != nil {
			return "", err
		}
		if topLevel == "" {
			topLevel = root
		} else if root != topLevel {
			return "", fmt.Errorf("GitHub repository archive contains multiple top-level roots")
		}
		if entry.UncompressedSize64 > uint64(maxExtractedFileBytes) {
			return "", fmt.Errorf("%w: archive entry %q exceeds the 20 MiB file limit", ErrArchiveTooLarge, entry.Name)
		}
		if entry.UncompressedSize64 > uint64(maxExtractedTotalBytes)-totalBytes {
			return "", fmt.Errorf("%w: exceeds the 100 MiB extracted-size limit", ErrArchiveTooLarge)
		}
		totalBytes += entry.UncompressedSize64

		target := filepath.Join(absoluteDestination, filepath.FromSlash(cleanName))
		relative, err := filepath.Rel(absoluteDestination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("archive entry %q escapes the extraction directory", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", fmt.Errorf("create archive directory %q: %w", cleanName, err)
			}
			continue
		}
		if err := extractRegularFile(entry, target); err != nil {
			return "", err
		}
	}

	repositoryRoot := filepath.Join(absoluteDestination, filepath.FromSlash(topLevel))
	info, err := os.Stat(repositoryRoot)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("GitHub repository archive root %q is not a directory", topLevel)
	}
	return repositoryRoot, nil
}

func validateArchiveEntry(entry *zip.File) (string, string, error) {
	if entry.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("archive entry %q is a symbolic link", entry.Name)
	}
	if !entry.FileInfo().IsDir() && !entry.Mode().IsRegular() {
		return "", "", fmt.Errorf("archive entry %q is not a regular file", entry.Name)
	}

	normalized := strings.ReplaceAll(entry.Name, "\\", "/")
	cleanName := path.Clean(normalized)
	if cleanName == "." || path.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
		return "", "", fmt.Errorf("archive entry %q has an unsafe path", entry.Name)
	}
	root := strings.Split(cleanName, "/")[0]
	if root == "" || root == "." || root == ".." {
		return "", "", fmt.Errorf("archive entry %q has an invalid top-level root", entry.Name)
	}
	return cleanName, root, nil
}

func extractRegularFile(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent for archive entry %q: %w", entry.Name, err)
	}
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open archive entry %q: %w", entry.Name, err)
	}
	defer source.Close()

	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create archive entry %q: %w", entry.Name, err)
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, maxExtractedFileBytes+1))
	closeErr := destination.Close()
	if copyErr != nil {
		return fmt.Errorf("extract archive entry %q: %w", entry.Name, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close archive entry %q: %w", entry.Name, closeErr)
	}
	if written > maxExtractedFileBytes {
		return fmt.Errorf("%w: archive entry %q exceeds the 20 MiB file limit", ErrArchiveTooLarge, entry.Name)
	}
	return nil
}
