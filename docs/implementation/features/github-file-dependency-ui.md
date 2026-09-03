# GitHub File Dependency Graph UI Plan

## Status

Implemented. This document defines the smallest feature required to enter one
public GitHub repository URL in the existing Files UI and display the resulting
file dependency graph.

## Goal

Allow a user to open the existing **Files** view, enter a public GitHub
repository URL, run the existing file analysis, and replace the demo fixture
with the returned normalized file graph.

Example input:

```text
https://github.com/thomxsnguyen/pc_diagnostic
```

The feature must reuse the current GitHub archive acquisition, language-neutral
file discovery, JavaScript/TypeScript, Python, and Go analyzers, in-memory queue,
and file-graph JSON schema.

## Scope Lock

### In scope

- One public `https://github.com/{owner}/{repository}` URL per analysis.
- One optional branch, tag, or commit ref.
- One compact repository form inside the existing Files sidebar.
- One same-origin HTTP endpoint that returns the completed normalized file
  graph in its response.
- Reuse of the current bounded GitHub ZIP download and safe extraction.
- Reuse of the current in-memory `analyze_file` queue and analyzer registry.
- Display of the returned graph in the existing `FileGraphCanvas`.
- Loading, validation, success, and error states for this request.
- Deterministic handler and frontend tests without live GitHub calls.

### Out of scope

- Package dependency analysis from this form.
- A combined package/file graph.
- Private repositories, GitHub OAuth, GitHub Apps, or token entry in the UI.
- GitHub Enterprise or other Git hosting providers.
- Multiple repositories, saved repository history, or recent-project lists.
- Repository search, browsing, forks, pull requests, or branch discovery.
- Local folder uploads or ZIP uploads.
- New language analyzers or changes to existing resolution behavior.
- Durable analysis jobs, PostgreSQL records, migrations, polling, retries, or
  resumable processing.
- WebSockets, server-sent events, or live per-file progress.
- Caching repository archives or graph results.
- Source-code viewing or editing.
- Authentication, deployment, hosting, or infrastructure changes.
- Redesign of the Dependencies view or unrelated UI components.

## Architectural Decision

Use a synchronous HTTP request around the existing in-memory analysis flow:

```text
Files sidebar form
       |
       | POST /api/file-graphs
       v
validate public GitHub URL and optional ref
       |
       v
existing bounded ZIP download and safe temporary extraction
       |
       v
existing repository discovery and immutable index
       |
       v
existing in-memory queue: one analyze_file job per supported file
       |
       v
existing analyzer registry: JavaScript/TypeScript, Python, Go
       |
       v
schemaVersion 1 file graph JSON
       |
       v
existing FileGraphCanvas
```

The HTTP request waits for the bounded analysis to finish and then returns the
graph. Do not introduce an API job ID, durable queue, polling endpoint, or
database storage for this demo feature.

The browser must not invoke the CLI or contact GitHub directly. GitHub access
and any optional `GITHUB_TOKEN` remain server-side.

## Reuse Boundary

The current file-analysis orchestration is located in `cmd/auditor`. Extract the
minimum reusable operation behind an internal service boundary so both the CLI
and HTTP handler call the same implementation.

Conceptual contract:

```go
type GitHubRequest struct {
    RepositoryURL string
    Ref           string
}

type Service interface {
    AnalyzeGitHub(ctx context.Context, request GitHubRequest) (Report, error)
}
```

The service owns:

- URL parsing through the existing GitHub URL validator;
- temporary-directory creation and cleanup;
- bounded archive download and safe extraction;
- repository discovery and module-index construction;
- analyzer-registry construction;
- the existing in-memory worker-pool lifecycle; and
- normalized report generation.

It does not write an output file. The CLI remains responsible for its
`--output` file, while the HTTP handler serializes the returned `Report`.

Do not change analyzer interfaces, queue interfaces, worker behavior, report
schema, or package-analysis behavior during this extraction.

## HTTP Contract

### Request

```http
POST /api/file-graphs
Content-Type: application/json
```

```json
{
  "repositoryUrl": "https://github.com/owner/repository",
  "ref": "main"
}
```

`ref` is optional. Omit it to use the repository's default branch.

Rules:

- Request bodies are bounded to a small fixed size.
- Unknown JSON fields are rejected.
- `repositoryUrl` is required and must pass the existing parser.
- Only HTTPS `github.com/{owner}/{repository}` URLs are accepted.
- Credentials, query strings, fragments, `/tree/`, and `/blob/` URLs remain
  invalid.
- Empty or whitespace-only refs are rejected when the field is supplied.

### Success response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "schemaVersion": 1,
  "root": "repository",
  "nodes": [],
  "edges": [],
  "diagnostics": []
}
```

The response is the existing `filegraph.Report` without a UI-specific wrapper.

### Error response

Use one small error shape:

```json
{
  "error": "The GitHub repository could not be analyzed."
}
```

Return bounded, user-safe messages. Never return tokens, authorization headers,
temporary paths, archive contents, or internal stack traces.

Expected status mapping:

- `400` — invalid JSON, repository URL, or ref.
- `404` — repository or ref not found.
- `413` — repository archive exceeds existing limits.
- `429` — GitHub rate limit or access restriction.
- `502` — GitHub network or unexpected upstream response failure.
- `500` — internal extraction or analysis failure.

The handler must propagate request cancellation into download and analysis.

## HTTP Process Boundary

Add one minimal Go HTTP process for the endpoint because the repository has no
current application server. It must:

- register only `POST /api/file-graphs` and a basic health route if required by
  the process itself;
- use standard `net/http` rather than adding a web framework;
- read `GITHUB_TOKEN` only from the server environment;
- apply request and shutdown timeouts;
- use dependency injection for the analysis service in handler tests; and
- avoid serving the frontend or defining production deployment in this feature.

For local development only, configure Vite to proxy `/api` to the local Go HTTP
process. Do not add CORS behavior when the browser can use the same-origin proxy.

## UI Placement

Add the form to the top of the existing `FileSidebar`, below the file totals and
above path search. Do not add a new page, modal, top-bar action, or repository
management panel.

Fields:

- **GitHub repository URL** — required URL input.
- **Ref** — optional text input for a branch, tag, or commit.
- **Analyze repository** — one primary action.

Keep the visual treatment consistent with the original UI plan:

- system typography and the existing restrained type scale;
- current neutral palette and blue accent;
- established spacing and sharp control treatment;
- no gradients, provider logos, decorative cards, or new color system; and
- existing focus, pressed, disabled, and reduced-motion behavior.

## Frontend Data Source

Keep `FixtureFileGraphDataSource` for the initial local demo graph. Add an HTTP
adapter implementing a narrow operation:

```ts
interface GitHubFileGraphRequest {
  repositoryUrl: string
  ref?: string
}

interface RepositoryFileGraphDataSource {
  analyze(request: GitHubFileGraphRequest): Promise<FileGraphSnapshot>
}
```

The adapter must:

- send `POST /api/file-graphs` with JSON;
- reject non-success responses using the server's safe error message;
- parse the success body through the existing `parseFileGraph` validator; and
- support cancellation through an `AbortSignal`.

Do not place GitHub API calls or tokens in frontend code.

## UI State and Behavior

Use local Files-view state only:

```text
idle -> submitting -> success
                  -> error
```

Behavior:

1. Dependencies remains the default graph mode.
2. Opening Files continues to show the bundled demo graph.
3. Submitting valid input disables the submit button and shows
   `Analyzing repository…` without removing the current graph.
4. A successful response replaces the current file snapshot.
5. Success resets file selection, path search, dragged positions, and viewport,
   then fits the returned graph.
6. The top-bar repository title changes to the report's `root` while Files is
   active.
7. An error leaves the previous graph visible and shows one inline message near
   the form.
8. Editing either input clears the prior form error.
9. A second submit aborts or is prevented while one request is active.
10. Leaving Files does not cancel a running analysis; returning shows its final
    success or error state.

Do not persist the URL, ref, returned graph, or file presentation state in
browser storage.

## Validation and Error Copy

Perform lightweight client validation for immediate feedback, but keep the
server authoritative.

Client errors:

- empty URL: `Enter a GitHub repository URL.`
- unsupported URL shape: `Use https://github.com/owner/repository.`
- supplied empty ref: `Enter a branch, tag, or commit, or leave Ref blank.`

Server and network errors appear in one inline `role="alert"` region. Avoid
toasts because the error is directly associated with the form.

## Security and Resource Limits

- Accept only the existing public GitHub URL shape.
- Keep `GITHUB_TOKEN` server-side and never return or log it.
- Preserve current compressed archive, extracted-size, per-file, and entry-count
  limits.
- Preserve safe extraction protections against traversal and special files.
- Run no repository code, package manager, compiler, or build script.
- Remove the temporary repository on success, failure, or cancellation.
- Apply one server-side analysis timeout.
- Do not add arbitrary URL fetching or user-selectable GitHub API hosts.

## Verification Plan

### Service and handler tests

- Valid public repository request returns schema version 1 JSON.
- Optional ref reaches the existing GitHub archive request.
- Invalid URL and ref return `400` before any upstream request.
- Not-found, size-limit, rate-limit, upstream, and analysis errors map to the
  documented status codes.
- Request cancellation reaches the service.
- Temporary extraction is cleaned up after success and failure.
- Tests use injected services or `httptest.Server`; they do not call GitHub.
- Existing CLI GitHub file-analysis tests continue to pass.

### Frontend tests

- The form appears only in Files mode.
- Empty and malformed URLs are rejected locally.
- Submitting sends the exact URL and optional ref.
- Submitting state disables duplicate requests.
- A valid response replaces the fixture and resets file presentation state.
- An invalid success payload is rejected by `parseFileGraph`.
- An error preserves the previous graph and renders inline feedback.
- Dependencies mode remains unchanged.

### Manual flow

1. Start the local Go HTTP process.
2. Start Vite with the local `/api` proxy.
3. Open Files.
4. Submit one small public JavaScript/TypeScript, Python, or Go repository.
5. Confirm the returned graph, counts, path search, selection, inspector, and
   diagnostics use the existing Files UI.

## Acceptance Criteria

- A user can submit one valid public GitHub repository URL from Files mode.
- The existing analysis pipeline processes the repository without requiring
  `DATABASE_URL`.
- The normalized result replaces the demo file graph in the existing canvas.
- The UI clearly communicates submitting, success, validation, and error states.
- GitHub credentials remain server-side.
- The existing package graph, package queue, report, and presentation editing
  behavior do not change.
- No durable jobs, database schema, polling protocol, private-repository flow,
  analyzer changes, or deployment work are introduced.
