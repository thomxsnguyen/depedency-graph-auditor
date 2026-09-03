# GitHub File Dependency Graph UI Implementation Plan

## Status

Implemented according to
[github-file-dependency-ui.md](github-file-dependency-ui.md).

## Objective

Connect the existing Files UI to the existing public-GitHub file-analysis flow.
A user submits one repository URL and optional ref, the server completes the
current in-memory file analysis, and the UI displays the returned
`schemaVersion: 1` graph.

## Hard Scope Boundary

This implementation includes only:

- extraction of the current file-analysis orchestration into a reusable
  internal service;
- one synchronous `POST /api/file-graphs` endpoint;
- one minimal local Go HTTP command;
- one repository form in the existing Files sidebar;
- one frontend HTTP adapter;
- replacement of the current file fixture after successful analysis; and
- tests directly covering those changes.

It does not include:

- package-analysis input or package graph changes;
- combined package/file graphs;
- private repositories or frontend token entry;
- durable jobs, PostgreSQL, migrations, job IDs, polling, or progress streams;
- changes to queue, worker, analyzer, discovery, or report contracts;
- new languages;
- saved repositories, caching, history, or persistence;
- source browsing or editing;
- deployment, hosting, containers, or CI changes; or
- unrelated UI redesign.

Do not add a Go web framework, frontend state library, validation library, or
other runtime dependency.

## Existing Behavior to Preserve

- Local and GitHub CLI file analysis continue to accept their current flags.
- `--analysis files` continues to work without `DATABASE_URL`.
- The CLI remains responsible for writing `--output` JSON.
- GitHub ZIP limits and safe extraction remain unchanged.
- File analysis continues to use one in-memory `analyze_file` job per discovered
  supported file.
- JavaScript/TypeScript, Python, and Go analyzers remain unchanged.
- `filegraph.Report` remains the only success response schema.
- Dependencies remains the default UI mode.
- The package graph, queue strip, report, presentation boxes, and saved package
  view remain unchanged.
- The bundled file fixture remains the initial Files demo.

## Implementation Shape

```text
web FileSidebar
  GitHubFileGraphForm
          |
          | POST /api/file-graphs
          v
cmd/filegraph-api
  net/http ServeMux
          |
          v
internal/httpapi.FileGraphHandler
          |
          v
internal/filegraph/service.Service
          |
          +-- internal/github archive download and extraction
          +-- existing file discovery and repository index
          +-- existing analyzer registry
          +-- existing in-memory queue and worker pool
          |
          v
filegraph.Report -> FileGraphSnapshot -> existing FileGraphCanvas
```

The HTTP request remains open until analysis succeeds, fails, times out, or is
cancelled. Do not create a background job after returning the response.

## Files to Add

```text
internal/filegraph/service/service.go
internal/filegraph/service/service_test.go
internal/httpapi/filegraph.go
internal/httpapi/filegraph_test.go
cmd/filegraph-api/main.go
cmd/filegraph-api/main_test.go
web/src/data/RepositoryFileGraphDataSource.ts
web/src/components/FileSidebar/GitHubFileGraphForm.tsx
web/tests/RepositoryFileGraphDataSource.test.ts
```

## Files to Change

```text
cmd/auditor/main.go
cmd/auditor/main_test.go
internal/github/archive.go               only if typed error classification is required
internal/github/archive_test.go          only for matching typed-error coverage
web/src/app/AuditWorkspace.tsx
web/src/components/FileSidebar/FileSidebar.tsx
web/src/components/TopBar/TopBar.tsx
web/src/styles/globals.css
web/src/styles/motion.css
web/vite.config.ts
web/tests/components.test.tsx
web/tests/browser/audit-studio.spec.ts
```

Do not modify database, migration, deployment, analyzer, queue, worker, or
package-manifest files.

## WP1: Extract Reusable File-Analysis Service

### Package placement

Create `internal/filegraph/service` rather than adding orchestration to the
root `filegraph` package. The analyzer implementations already import
`filegraph`; having `filegraph` import its analyzer subpackages would create an
import cycle.

### Service contract

Define the smallest caller-facing operations needed by the CLI and HTTP layer:

```go
type GitHubRequest struct {
    RepositoryURL string
    Ref           string
}

type Options struct {
    WorkerCount    int
    ShutdownTimeout time.Duration
}

type Service struct {
    // injected GitHub archive client and bounded options
}

func (s *Service) AnalyzeGitHub(
    ctx context.Context,
    request GitHubRequest,
) (filegraph.Report, error)

func (s *Service) AnalyzeDirectory(
    ctx context.Context,
    root string,
    reportRoot string,
) (filegraph.Report, error)
```

Use defaults matching the current CLI: ten workers and the existing shutdown
timeout. Reject zero or invalid configuration through constructor validation or
documented defaults; do not expose arbitrary worker configuration through HTTP.

### GitHub acquisition

`AnalyzeGitHub` must:

1. Parse `RepositoryURL` through `github.ParseRepositoryURL`.
2. Reject a supplied empty or whitespace-only ref.
3. Create one application-owned temporary directory.
4. Download one repository ZIP through the existing client.
5. Extract it through `github.ExtractRepositoryZIP`.
6. Call `AnalyzeDirectory` with the extracted root and repository name.
7. Remove the temporary directory on every exit path.

The service receives a GitHub client interface exposing only the archive method
needed by this workflow. Production uses `github.GitHubClient`; tests inject a
fake or an `httptest` client. The server supplies `GITHUB_TOKEN` from its
environment.

### Directory analysis

Move the reusable body of the current `executeFileAnalysis` into
`AnalyzeDirectory` without changing its behavior:

- discover the repository once;
- build the immutable repository index once;
- build the existing Go module index;
- prepopulate all normalized file nodes;
- construct the existing analyzer registry;
- create the current in-memory queue;
- submit one current `analyze_file` job per discovered path;
- run the existing worker pool;
- generate one deterministic `filegraph.Report`; and
- return the report without writing a file.

The request context controls GitHub I/O and analysis cancellation. Shutdown is
still bounded by the configured timeout. Do not replace the in-memory queue or
alter worker retry behavior.

### CLI adaptation

Update `cmd/auditor` to call the service for local and GitHub file analysis.
After the service returns, keep the existing CLI-only output write and terminal
summary in `cmd/auditor`.

Preserve CLI signal handling by passing its existing signal-aware context into
the service. Do not change package-analysis orchestration.

### WP1 checks

- Local directory analysis returns the same deterministic report.
- GitHub analysis uses the requested ref and repository name.
- Temporary content is removed after success and failure.
- Cancellation stops the run and bounded shutdown completes.
- Empty repositories return valid empty arrays.
- Existing CLI file-analysis tests pass without output changes.

## WP2: Add the HTTP Boundary

### Handler contract

Create `internal/httpapi/filegraph.go` with an injected interface:

```go
type FileGraphAnalyzer interface {
    AnalyzeGitHub(
        ctx context.Context,
        request service.GitHubRequest,
    ) (filegraph.Report, error)
}
```

The handler supports only:

```http
POST /api/file-graphs
Content-Type: application/json
```

Request body:

```json
{
  "repositoryUrl": "https://github.com/owner/repository",
  "ref": "main"
}
```

Implementation requirements:

- reject methods other than `POST` with `405` and an `Allow: POST` header;
- require JSON content type;
- cap the request body at 4 KiB;
- decode exactly one JSON object;
- reject unknown fields and trailing JSON values;
- validate the URL before invoking the analyzer;
- reject an explicitly supplied blank ref;
- call the analyzer with `request.Context()`;
- set `Content-Type: application/json` for every response; and
- encode the existing `filegraph.Report` directly on success.

Error body:

```json
{
  "error": "The GitHub repository could not be analyzed."
}
```

Do not return raw internal errors to clients. Log server-side detail without
tokens, authorization headers, temporary paths, or response bodies.

### Error classification

Classify errors without matching human-readable strings. Add the smallest typed
error or sentinel support needed at the GitHub archive/service boundary.

Map:

- request decode and validation errors to `400`;
- repository or ref not found to `404`;
- archive limit errors to `413`;
- GitHub forbidden or rate-limit responses to `429`;
- GitHub transport and unexpected upstream status errors to `502`;
- remaining internal extraction or analysis errors to `500`.

Preserve existing CLI error text where tests depend on it. Do not refactor the
manifest-fetch error model or unrelated clients.

### HTTP command

Create `cmd/filegraph-api/main.go` using standard `net/http` only.

The command must:

- listen on `127.0.0.1:8080` by default;
- optionally accept `FILEGRAPH_API_ADDR` for local configuration;
- read `GITHUB_TOKEN` from the environment;
- construct the production file-graph service and handler;
- register only `/api/file-graphs`;
- configure header, read, write, idle, and shutdown timeouts;
- handle `SIGINT` and `SIGTERM`; and
- perform graceful shutdown with a fixed bounded timeout.

Do not serve frontend assets, enable CORS, add authentication, or define
production deployment.

### WP2 checks

- Valid request returns `200` and the exact report JSON.
- Invalid method, content type, body size, JSON, URL, and ref fail before
  analysis.
- Each typed service failure maps to the documented status and safe body.
- Request cancellation reaches the injected analyzer.
- The command starts and shuts down using injected or testable construction
  where needed; tests do not bind a public port.

## WP3: Add the Frontend HTTP Adapter

Create `RepositoryFileGraphDataSource.ts` with:

```ts
export interface GitHubFileGraphRequest {
  repositoryUrl: string
  ref?: string
}

export interface RepositoryFileGraphDataSource {
  analyze(
    request: GitHubFileGraphRequest,
    signal?: AbortSignal,
  ): Promise<FileGraphSnapshot>
}
```

Production behavior:

- `POST` to `/api/file-graphs`;
- set `Content-Type: application/json`;
- trim the URL and optional ref;
- omit `ref` when the user leaves it blank;
- parse successful JSON through the existing `parseFileGraph` function;
- read the bounded server error shape for non-success responses;
- fall back to one generic network error when no safe server message exists;
  and
- forward the optional abort signal to `fetch`.

Do not call `api.github.com` from the browser. Do not add a GitHub token field or
store request values.

### Vite development proxy

Update only the Vite development server configuration:

```ts
server: {
  proxy: {
    "/api": "http://127.0.0.1:8080",
  },
}
```

This is local development wiring, not a deployment definition. Do not add CORS
headers or production proxy configuration.

### WP3 checks

- Request method, path, headers, and body are exact.
- Blank optional ref is omitted.
- Valid response is schema-validated.
- Invalid success JSON is rejected.
- Safe server errors are exposed to the form.
- Network and abort errors remain distinct.

## WP4: Add the Scoped Files-Sidebar Form

### Form component

Create `GitHubFileGraphForm.tsx` and render it only from `FileSidebar`.

Props:

```ts
interface GitHubFileGraphFormProps {
  submitting: boolean
  error: string | null
  onSubmit(request: GitHubFileGraphRequest): void
  onChange(): void
}
```

Fields and action:

- required `type="url"` GitHub repository URL;
- optional Ref text input;
- one `Analyze repository` button;
- inline submitting label `Analyzing repository…`; and
- one inline `role="alert"` error region.

Client validation must accept only the existing repository URL shape and show:

- `Enter a GitHub repository URL.` for empty input;
- `Use https://github.com/owner/repository.` for an invalid shape; and
- `Enter a branch, tag, or commit, or leave Ref blank.` only when a supplied ref
  consists entirely of whitespace.

Use a small frontend validation function; do not add a validation package.
Server validation remains authoritative.

### Visual rules

- Place the form below file totals and above path search.
- Reuse current input, button, spacing, focus, and error styles.
- Keep the sidebar width and current responsive drawer/sheet behavior.
- Use no GitHub logo, new card surface, gradient, modal, or toast.
- Disable all form fields and the action while submitting.
- Preserve visible focus and reduced-motion behavior.

### WP4 checks

- The form appears in Files and not Dependencies.
- Labels and errors are accessible.
- Pointer and keyboard submission work.
- Duplicate submission is impossible while pending.
- Editing either field clears the previous form error.
- The existing file path search remains independent.

## WP5: Connect Repository Analysis to the Files Workspace

Add only these local states to `AuditWorkspace`:

```ts
const [repositoryStatus, setRepositoryStatus] =
  useState<"idle" | "submitting" | "success" | "error">("idle")
const [repositoryError, setRepositoryError] = useState<string | null>(null)
```

Keep one `AbortController` ref for application unmount cleanup. Changing graph
mode must not abort a pending request.

Submission flow:

1. Clear the previous repository error.
2. Set status to `submitting`.
3. Call `RepositoryFileGraphDataSource.analyze`.
4. On success, replace `fileGraph` with the validated snapshot.
5. Clear selected file, file search, dragged file positions, and file viewport.
6. Set status to `success`; `FileGraphCanvas` performs its normal initial fit.
7. On non-abort failure, preserve the previous graph and display the safe inline
   error.
8. On application unmount, abort the request without showing an error.

While Files is active, use `fileGraph.root` as the top-bar title after the
fixture or repository graph has loaded. Dependencies continues using the
package snapshot root.

Do not change `GraphView`, `LocalGraphViewStore`, package reducers, or queue
state. Do not persist the repository request or response.

### WP5 checks

- The demo fixture remains visible before the first submission.
- Submitting does not blank or replace the graph with a global loader.
- Success replaces nodes, edges, counts, diagnostics, sidebar data, and
  inspector source together.
- Success resets only file presentation state.
- Failure preserves the previous file graph.
- Switching to Dependencies during a request leaves package behavior unchanged.
- Returning to Files shows the completed result or inline error.

## WP6: Tests and Regression Verification

### Go unit tests

Add deterministic coverage for:

- service URL/ref validation;
- GitHub archive acquisition and temporary cleanup;
- directory analysis through the existing queue;
- service cancellation and bounded shutdown;
- handler request validation and method handling;
- handler success JSON and safe error JSON;
- typed HTTP status mapping; and
- server construction and graceful shutdown seams.

Use temporary directories, local fixtures, fake services, and
`httptest.Server`. Do not contact GitHub.

### Existing Go regressions

Run:

```bash
go test ./...
```

Confirm package audits and all existing local/GitHub file-analysis tests remain
unchanged.

### Frontend unit and component tests

Cover:

- adapter request and response parsing;
- local URL/ref validation;
- submitting and disabled form state;
- inline errors and error clearing;
- successful graph replacement and file-state reset;
- failed analysis preserving the prior graph;
- form visibility by graph mode; and
- top-bar root switching only in Files mode.

Run:

```bash
cd web
npm test
npm run lint
npm run build
```

### Browser regression

Mock `/api/file-graphs` in Playwright; do not call GitHub. Verify:

1. Open Files.
2. Submit a public-repository-shaped URL.
3. Observe the submitting state.
4. Fulfill the mocked normalized report.
5. Confirm the graph, totals, title, search, selection, and inspector use the
   returned report.
6. Repeat with a mocked error and confirm the prior graph remains visible.
7. Run the existing Dependencies and presentation-box browser tests unchanged.

## Local Manual Verification

Run the two local processes in separate terminals:

```bash
go run ./cmd/filegraph-api
```

```bash
cd web
npm run dev
```

Then open Files and submit one small public repository. `GITHUB_TOKEN` may be
set only in the Go server environment to reduce public API rate-limit failures.
`DATABASE_URL` must not be required.

## Completion Criteria

The feature is complete only when:

- one public GitHub repository URL and optional ref can be submitted from Files;
- the HTTP endpoint returns the existing normalized file-graph schema;
- the CLI and endpoint share one file-analysis service implementation;
- the existing in-memory queue and analyzer registry perform the analysis;
- the returned graph replaces the fixture without a page reload;
- submitting, validation, success, network, and server-error states are clear;
- credentials and temporary paths never reach frontend responses or logs;
- all temporary repository content is cleaned up;
- Go tests, frontend tests, lint, build, and mocked browser coverage pass; and
- no package-graph, durable-job, database, analyzer, private-repository,
  deployment, or unrelated UI changes are included.
