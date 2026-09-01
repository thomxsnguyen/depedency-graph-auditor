# File Dependency Graph — Implementation Plan

## Status

Implemented. This document records the completed, scoped implementation.

## Objective

Add a local JavaScript/TypeScript file-import analysis mode that runs one source
file per job through the existing queue and worker pool and writes a
deterministic JSON graph.

The graph represents actual local imports:

```text
src/App.tsx -> src/components/Button.tsx
```

It does not represent external package dependencies or relationships that are
only theoretically possible.

## Separation From Package Dependencies

The two dependency graphs remain separate:

| Concern | Existing package graph | New file graph |
|---|---|---|
| Node identity | Package name and version | Project-relative source path |
| Edge meaning | Package depends on package | File imports local file |
| Input | Dependency manifest | Local project directory |
| Job type | `audit_package` | `analyze_file` |
| Handler | `auditor.AuditHandler` | `filegraph.Handler` |
| Result store | `PackageStore` and `EdgeStore` | `filegraph.Store` |
| Output | Existing audit report | File graph JSON |

The only shared components are the generic `job.Job`, queue, worker pool,
retry, DLQ, and shutdown infrastructure. File-graph data must not be inserted
into the package stores, and package graph data must not appear in file-graph
output.

## Fixed Scope

### Included

- One local project directory per run.
- `.js`, `.jsx`, `.ts`, and `.tsx` files.
- Relative `./` and `../` imports.
- Static imports, side-effect imports, re-exports, `require()`, and
  string-literal dynamic imports.
- One queue job for every discovered source file.
- Nodes, resolved edges, and unresolved-import diagnostics.
- Deterministic JSON output.
- Unit tests and one local integration test.

### Excluded

- UI and deployment changes.
- Queue, worker, retry, DLQ, or database schema changes.
- Changes to existing package-audit results or behavior.
- External package imports.
- TypeScript path aliases and bundler aliases.
- GitHub repository traversal.
- Additional languages.
- Call graphs, data flow, symbol analysis, and architecture rules.
- Watch mode, caching, and incremental analysis.
- Durable persistence for file graph results.

## Files to Add

```text
internal/filegraph/
├── types.go
├── store.go
├── store_test.go
├── discovery.go
├── discovery_test.go
├── extractor.go
├── extractor_test.go
├── resolver.go
├── resolver_test.go
├── handler.go
├── handler_test.go
├── report.go
└── report_test.go
```

## Files to Modify

```text
cmd/auditor/main.go
cmd/auditor/main_test.go
cmd/auditor/main_integration_test.go
```

Do not modify:

```text
internal/job/
internal/queue/
internal/worker/
internal/dlq/
internal/store/
internal/auditor/
db/migrations/
web/
```

## Step 1 — Define File Graph Types

Add `internal/filegraph/types.go`:

```go
type Node struct {
    Path string `json:"path"`
}

type Edge struct {
    From string `json:"from"`
    To   string `json:"to"`
}

type Diagnostic struct {
    Path    string `json:"path"`
    Import  string `json:"import,omitempty"`
    Message string `json:"message"`
}

type Report struct {
    Root        string       `json:"root"`
    Nodes       []Node       `json:"nodes"`
    Edges       []Edge       `json:"edges"`
    Diagnostics []Diagnostic `json:"diagnostics"`
}
```

Use normalized, slash-separated, project-relative paths as identities. Do not
reuse package names, versions, licenses, or verdicts.

## Step 2 — Add the Concurrent Result Store

Add `internal/filegraph/store.go` with:

- a mutex-protected node set keyed by path;
- a mutex-protected edge set keyed by `(from, to)`;
- an append-only diagnostics collection;
- snapshot methods that return copies rather than internal collections.

Required behavior:

1. Every discovered file is added as a node before workers start.
2. Duplicate nodes and exact duplicate edges are ignored.
3. Concurrent worker writes are safe.
4. Snapshot ordering is not relied upon; the report layer owns sorting.

Complete `store_test.go` before continuing.

## Step 3 — Discover Source Files

Add `internal/filegraph/discovery.go`:

```go
type Index map[string]struct{}

func Discover(root string) ([]string, Index, error)
```

Implementation requirements:

1. Convert the input root to an absolute, cleaned path.
2. Require it to be an existing directory.
3. Walk it once.
4. Skip symlinks.
5. Skip `.git`, `node_modules`, `dist`, `build`, and `coverage` directories.
6. Include only regular `.js`, `.jsx`, `.ts`, and `.tsx` files.
7. Convert paths to slash-separated paths relative to the root.
8. Sort the returned paths.
9. Build an immutable lookup index from the same paths.

Tests must verify filtering, deterministic ordering, and symlink exclusion.

## Step 4 — Extract Supported Imports

Add `internal/filegraph/extractor.go`:

```go
func ExtractImports(source []byte) ([]string, error)
```

Recognize only string-literal specifiers used by:

```ts
import value from "./value";
import "./setup";
export { value } from "./value";
export * from "./value";
require("./value");
import("./value");
```

Return relative specifiers beginning with `./` or `../`. Ignore bare package
imports, URLs, computed expressions, and import-like text inside comments.

Use a focused tokenizer or parser. Do not implement extraction as unrestricted
regular-expression matching over the complete file.

Tests must be table-driven and cover each supported form and each explicit
exclusion.

## Step 5 — Resolve Imports Through the File Index

Add `internal/filegraph/resolver.go`:

```go
func Resolve(index Index, importer, specifier string) (string, bool)
```

For a relative specifier, resolve from the importer's directory in this exact
order:

1. Exact path with a supported extension.
2. `.ts`.
3. `.tsx`.
4. `.js`.
5. `.jsx`.
6. `/index.ts`.
7. `/index.tsx`.
8. `/index.js`.
9. `/index.jsx`.

Normalize the result and reject paths that escape the project root. Return
`false` when no indexed file matches. Do not access the filesystem during
resolution.

## Step 6 — Implement the File Job Handler

Add `internal/filegraph/handler.go` implementing the existing `job.Handler`:

```go
const JobType = "analyze_file"

type Payload struct {
    Root string `json:"root"`
    Path string `json:"path"`
}
```

For each job:

1. Require `j.Type == JobType`.
2. Decode and validate the payload.
3. Safely join `Root` and `Path`; reject absolute and escaping paths.
4. Read the file through an injectable reader with a fixed maximum size.
5. Extract imports.
6. Resolve relative imports through the shared read-only index.
7. Add resolved edges to the file graph store.
8. Add diagnostics for unresolved relative imports.
9. Return no child jobs.

File read failures return errors and therefore use the existing retry/DLQ
behavior. Deterministic extraction failures are added as diagnostics and return
success so retries are not wasted on unchanged source.

## Step 7 — Generate Deterministic JSON

Add `internal/filegraph/report.go`:

```go
func GenerateReport(root string, store *Store) Report
func MarshalReport(report Report) ([]byte, error)
```

The report layer must:

- sort nodes by path;
- sort edges by `from`, then `to`;
- sort diagnostics by path, import, then message;
- preserve exact edge deduplication;
- use project-relative paths;
- produce indented UTF-8 JSON ending in one newline.

The CLI writes the output only after successful pool shutdown.

## Step 8 — Add the CLI Mode

Modify `cmd/auditor/main.go` with one option:

```text
--analysis packages|files
```

Default: `packages`.

File mode command:

```bash
go run ./cmd/auditor \
  --analysis files \
  --output file-graph.json \
  ./personal-portfolio
```

File mode orchestration:

1. Validate that the input is a local directory and `--output` is present.
2. Discover files and construct the index.
3. Create the file graph store and prepopulate all nodes.
4. Create the existing queue and worker pool with `filegraph.Handler`.
5. Submit one `analyze_file` job for each discovered path.
6. Start, await completion, and shut down using the existing lifecycle.
7. Generate and write the JSON report.

Keep the existing package-mode branch unchanged. Do not add a job router or run
package and file analysis in the same invocation.

## Step 9 — Test the Complete Flow

Add one integration fixture at test runtime containing:

```text
src/App.tsx              -> ./components/Button
src/components/Button.tsx -> ../App
src/orphan.ts             -> no imports
src/broken.ts             -> ./missing
node_modules/ignored.js   -> ignored
```

Run it through the real queue and worker pool and assert:

- all four supported project files appear as nodes;
- both cycle edges appear exactly once;
- the orphan file remains present;
- the missing import becomes one diagnostic;
- the ignored dependency directory is absent;
- output ordering is deterministic.

Existing CLI and auditor tests must continue to pass unchanged.

## Step 10 — Verification

Run:

```bash
gofmt -w cmd/auditor internal/filegraph
go test ./internal/filegraph
go test -race ./internal/filegraph
go test ./...
```

Then run the file graph command twice against the same fixture and verify that
the generated JSON files are byte-identical.

## Completion Checklist

- [x] Package analysis remains the default CLI mode.
- [x] File mode is explicit and local-directory-only.
- [x] Only JavaScript and TypeScript source files are analyzed.
- [x] Only resolved relative imports become edges.
- [x] External packages never become file nodes.
- [x] One file corresponds to one queue job.
- [x] No child jobs are produced by the file handler.
- [x] Cycles and disconnected files complete correctly.
- [x] Output is deterministic.
- [x] Queue and worker interfaces are unchanged.
- [x] No database migration is added.
- [x] No UI or deployment file is changed.
- [x] All existing tests pass.
