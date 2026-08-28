# Feature Implementation — Public GitHub Repository Input

## Status

Proposed feature. This document defines the implementation boundary for reading
one npm manifest from a public GitHub repository. It does not modify the current
auditor.

## Goal

Allow a user to provide a public GitHub repository URL instead of downloading
or manually copying its `package.json`:

```bash
go run ./cmd/auditor \
  --output audit-report.md \
  https://github.com/owner/repository
```

The downloaded manifest becomes the existing audit seed. npm resolution,
durable job processing, policy evaluation, dependency mapping, and Markdown
report generation remain unchanged.

## Scope

### In scope

- Accept a public `https://github.com/{owner}/{repository}` URL as the single
  positional input.
- Preserve the existing local `package.json` input.
- Read one repository manifest through the GitHub Contents API.
- Default to `package.json` in the repository root.
- Accept an optional repository ref and manifest path.
- Support an optional `GITHUB_TOKEN` for authenticated API requests.
- Parse the downloaded manifest in memory without cloning the repository.
- Return clear URL, HTTP, rate-limit, size, and manifest errors.
- Add deterministic tests using `httptest.Server`; tests must not call GitHub.

### Out of scope

- Private repositories.
- GitHub Enterprise hosts.
- Repository cloning or archive downloads.
- Automatic monorepo or workspace discovery.
- Auditing multiple manifests in one command.
- Lockfile parsing.
- Git submodules.
- Other Git hosting providers.
- GitHub webhooks, apps, OAuth flows, or device authentication.
- Caching GitHub responses.
- Changes to npm resolution, graph rendering, queue, worker, retry, DLQ, or
  shutdown behavior.

## CLI Contract

The positional input accepts either a local file or a public GitHub repository:

```bash
go run ./cmd/auditor [options] <package.json-path-or-github-url>
```

Existing local behavior remains valid:

```bash
go run ./cmd/auditor \
  --output audit-report.md \
  ./package.json
```

Public repository root manifest:

```bash
go run ./cmd/auditor \
  --output audit-report.md \
  https://github.com/owner/repository
```

Manifest on a branch, tag, commit, or in a subdirectory:

```bash
go run ./cmd/auditor \
  --ref development \
  --manifest packages/web/package.json \
  --output audit-report.md \
  https://github.com/owner/repository
```

New options:

| Option | Default | Behavior |
|---|---|---|
| `--ref <value>` | Repository default branch | Select a branch, tag, or commit |
| `--manifest <path>` | `package.json` | Select one repository-relative manifest |

Rules:

- exactly one positional input remains required;
- `--ref` and `--manifest` are valid only with GitHub input;
- empty option values are invalid;
- local-file behavior and `--output` remain unchanged;
- URL path forms such as `/tree/...` and `/blob/...` are not inferred;
- callers use `--ref` and `--manifest` explicitly to avoid ambiguous parsing.

## Accepted GitHub URL

Accept only this repository URL shape:

```text
https://github.com/{owner}/{repository}
```

A trailing slash and `.git` repository suffix may be normalized. Reject:

- non-HTTPS schemes;
- hosts other than `github.com`;
- missing owner or repository segments;
- extra path segments;
- embedded credentials;
- query strings and fragments.

URL parsing must use `net/url`, not string splitting.

## Manifest Path Validation

`--manifest` is a repository-relative POSIX path. It must:

- be non-empty;
- not begin with `/`;
- not contain `.` or `..` traversal segments;
- identify one file path;
- be escaped as an API path without changing its logical segments.

The first implementation does not search for a manifest when the requested path
does not exist.

## GitHub API Contract

Fetch repository contents with:

```http
GET https://api.github.com/repos/{owner}/{repository}/contents/{manifest}
Accept: application/vnd.github.raw+json
```

When `--ref` is present, append it as a URL-encoded query parameter:

```text
?ref={branch-tag-or-commit}
```

When no ref is supplied, GitHub uses the repository default branch.

Public repository content can be requested without authentication. If
`GITHUB_TOKEN` is set, add:

```http
Authorization: Bearer <token>
```

The token must never be printed, included in an error, persisted, or placed in
the request URL.

Reference:

- <https://docs.github.com/en/rest/repos/contents>
- <https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api>
- <https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api>

## Source Boundary

Introduce a source result that is independent of local files and GitHub:

```go
type ManifestSource struct {
    Location string
    Data     []byte
}
```

Source selection is owned by the CLI orchestration layer:

```text
positional input
├── valid GitHub URL → GitHub manifest fetcher
└── otherwise        → local manifest reader
```

The GitHub client only retrieves bytes. It does not parse npm dependencies,
open PostgreSQL, or invoke the auditor.

## Parser Refactor

The dependency parser currently owns local file opening. Add an in-memory
boundary so both sources use the same JSON parsing logic:

```go
func ParsePackageJSON(reader io.Reader, includeDevDeps bool) (Manifest, error)
```

The returned manifest contains at least:

```go
type Manifest struct {
    Name         string
    Dependencies []Dependency
}
```

Keep a local-path wrapper if useful for compatibility:

```go
func ParsePackageJSONFile(path string, includeDevDeps bool) (Manifest, error)
```

The manifest name supplies the audit root. This replaces the CLI's separate
second read of the local file and gives local and GitHub inputs identical root
name behavior.

Dependency ordering must remain deterministic after parsing.

## GitHub Client

Add a small client with an injectable base URL and HTTP client:

```go
type GitHubClient struct {
    HTTPClient *http.Client
    BaseURL    string
    Token      string
}

func (c *GitHubClient) FetchManifest(
    ctx context.Context,
    repository Repository,
    manifestPath string,
    ref string,
) ([]byte, error)
```

Production defaults:

- base URL: `https://api.github.com`;
- request timeout: 15 seconds;
- maximum manifest response: 1 MiB;
- no automatic retries in the initial implementation.

The injectable base URL exists only for deterministic HTTP tests.

## Execution Flow

The CLI sequence becomes:

1. Parse CLI arguments.
2. Classify the positional input as local or GitHub.
3. Read or fetch the selected manifest.
4. Parse its project name, dependencies, and development dependencies.
5. Reject an invalid or missing manifest before opening PostgreSQL.
6. Run the existing audit lifecycle.
7. Generate terminal and optional Markdown reports as currently implemented.

The GitHub request is a preflight input operation. It is not a queue job and
does not participate in npm retry or DLQ behavior.

## Error Handling

Return clear errors for:

- malformed or unsupported GitHub URLs;
- missing owner or repository;
- invalid manifest paths;
- empty `--ref` or `--manifest` values;
- use of GitHub-only options with a local file;
- DNS, connection, TLS, or timeout failures;
- `404 Not Found` repository, ref, or manifest responses;
- `403 Forbidden` and `429 Too Many Requests` rate-limit responses;
- other non-success HTTP status codes;
- responses larger than the configured limit;
- empty or invalid JSON manifests;
- manifests without dependencies, preserving the current no-work behavior.

HTTP errors should identify the public repository and manifest path but must not
include credentials or response bodies.

## Security Constraints

- Allow only `https://github.com` as user input.
- Construct API URLs from validated owner, repository, path, and ref values.
- Never fetch an arbitrary user-provided hostname.
- Apply a fixed request timeout and response-size limit.
- Do not execute repository code.
- Do not clone, install, build, or run package scripts.
- Do not write downloaded contents to disk.
- Read `GITHUB_TOKEN` from the environment only.
- Never log request authorization headers.

These constraints keep repository input read-only and prevent the feature from
becoming an arbitrary URL fetcher.

## Component Changes

```text
cmd/auditor/
├── main.go                         ← source selection and new CLI options
└── main_test.go                    ← argument and source-selection tests
internal/depfile/
├── depfile.go                      ← reader-based manifest parsing
└── depfile_test.go                 ← shared local/in-memory parser tests
internal/github/
├── client.go                       ← public Contents API client
└── client_test.go                  ← deterministic httptest coverage
```

No auditor handler, npm registry client, queue, worker, store schema, or
Markdown renderer changes are required.

## Testing Strategy

### URL and CLI tests

| Test | Verification |
|---|---|
| Local path | Existing positional input remains valid |
| Public repository | Canonical GitHub URL is accepted |
| `.git` suffix | Repository name is normalized |
| Trailing slash | Canonical repository is preserved |
| Invalid host | Non-GitHub host is rejected |
| Invalid scheme | HTTP and non-web schemes are rejected |
| Extra URL path | `/tree` and `/blob` forms are rejected |
| Local-only conflict | GitHub options with a local path are rejected |
| Missing values | Empty ref and manifest options return clear errors |

### GitHub client tests

| Test | Verification |
|---|---|
| Default branch | No `ref` query is sent |
| Explicit ref | Ref is encoded as a query parameter |
| Nested manifest | Repository-relative path is escaped correctly |
| Public request | Request succeeds without authorization |
| Token request | Bearer header is set without leaking the token |
| Raw response | Manifest bytes are returned unchanged |
| Not found | `404` produces a repository/path-specific error |
| Rate limited | `403` and `429` produce clear rate-limit errors |
| Oversized body | Response exceeding 1 MiB is rejected |
| Timeout | Context or client timeout is propagated clearly |

### Parser and orchestration tests

| Test | Verification |
|---|---|
| Reader parser | Local and downloaded bytes produce equivalent manifests |
| Project root | Manifest `name` becomes the report root |
| Dev dependencies | Existing inclusion behavior remains unchanged |
| Invalid JSON | Failure occurs before PostgreSQL is opened |
| Empty graph | Existing no-dependency behavior remains unchanged |
| GitHub end to end | Mock Contents API data reaches existing audit seeding |

Tests must use temporary files and `httptest.Server`. They must not require a
GitHub account, GitHub token, or public network access.

## Validation Commands

```bash
go test -race ./internal/depfile ./internal/github ./cmd/auditor
go test -race ./...
go test -race -tags=integration ./...
```

One optional manual smoke test may use a small public repository after all
deterministic tests pass.

## Exit Criteria

- [ ] Existing local `package.json` input behaves as before.
- [ ] A public GitHub repository URL loads its root `package.json`.
- [ ] `--ref` selects an explicit branch, tag, or commit.
- [ ] `--manifest` selects one nested repository manifest.
- [ ] Repository content is parsed without cloning or temporary files.
- [ ] The downloaded manifest name becomes the audit root.
- [ ] GitHub errors are clear and do not expose tokens.
- [ ] URL validation prevents arbitrary-host requests.
- [ ] Response timeout and size limits are enforced.
- [ ] npm resolution and graph/report behavior remain unchanged.
- [ ] No database migration is added.
- [ ] `go test -race ./...` passes.
- [ ] `go test -race -tags=integration ./...` passes against PostgreSQL.
