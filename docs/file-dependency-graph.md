# Feature Implementation Plan — JavaScript/TypeScript File Dependency Graph

## Status

Proposed feature. This document defines the implementation boundary only; it
does not modify the current dependency auditor.

## Goal

Add an optional analysis mode that scans a local JavaScript or TypeScript
project and produces a directed graph of its actual local file imports.

For an import such as:

```ts
import { Button } from "./components/Button";
```

the graph records:

```text
src/App.tsx -> src/components/Button.tsx
```

The feature must reuse the existing durable queue, bounded worker pool, retry,
DLQ, and shutdown behavior without changing their interfaces or semantics.

## Scope

### In scope

| Concern | Required behavior |
|---|---|
| Input | Scan one local project directory |
| Languages | JavaScript and TypeScript: `.js`, `.jsx`, `.ts`, and `.tsx` |
| Relationships | Record local, relative imports between project files |
| Import forms | Static imports, re-exports, `require()`, and string-literal dynamic imports |
| Queue usage | Submit one `analyze_file` job for every discovered source file |
| Concurrency | Process file jobs with the existing bounded worker pool |
| Graph | Record every discovered source file as a node and every resolved local import as an edge |
| Diagnostics | Record unresolved local imports and source parsing failures without inventing edges |
| Output | Write one deterministic JSON file containing nodes, edges, and diagnostics |
| Tests | Cover discovery, extraction, resolution, graph storage, output, and CLI behavior |

### Out of scope

- UI changes or visualization work
- Deployment changes
- Changes to queue, worker, retry, DLQ, store, or shutdown contracts
- Package dependency or license-audit behavior changes
- Python, Go, Java, Kotlin, or other source languages
- External package imports such as `react` or `express`
- TypeScript `paths`, bundler aliases, or framework-specific aliases
- Runtime call graphs, function calls, data flow, or class relationships
- Inferring files that could theoretically depend on each other
- Repository cloning or GitHub directory traversal
- Watch mode or incremental analysis
- New PostgreSQL tables or migrations
- Persisting file graph nodes and edges between process runs
- Circular-dependency policy enforcement
- Unused-file or architecture-rule enforcement

## Proposed CLI Contract

Keep package auditing as the default behavior and require an explicit analysis
mode for file graphs:

```bash
go run ./cmd/auditor \
  --analysis files \
  --output file-graph.json \
  ./personal-portfolio
```

Rules:

- `--analysis` accepts `packages` or `files` and defaults to `packages`;
- file analysis requires one local directory input;
- file analysis requires a non-empty `--output` path;
- existing package-audit commands remain unchanged;
- GitHub URLs are rejected in file mode for the initial implementation;
- unsupported files are ignored rather than submitted as jobs.

## Execution Flow

```text
local project directory
        |
        v
discover supported source files and build a file index
        |
        v
submit one analyze_file job per source file
        |
        v
existing queue -> existing worker pool -> FileGraphHandler
        |                                  |
        |                                  v
        |                          extract and resolve imports
        |                                  |
        +----------------------------------v
                              thread-safe graph store
                                         |
                                         v
                              deterministic JSON report
```

The discovery step seeds all supported files. The handler does not enqueue
child jobs. This guarantees that disconnected and unreferenced files appear in
the graph, while cycles require no special traversal or deduplication logic.

## Job Contract

Use the existing generic `job.Job` with a new type:

```text
analyze_file
```

Payload:

```go
type AnalyzeFilePayload struct {
    Root string `json:"root"`
    Path string `json:"path"`
}
```

`Root` is the validated absolute project root used for safe file access.
`Path` is a normalized, slash-separated path relative to that root. The
handler must reject absolute paths and paths that escape the root.

The worker pool continues to receive one `job.Handler`. In file mode the CLI
constructs a `FileGraphHandler`; in package mode it continues to construct the
existing `AuditHandler`. A job-type router is not required.

## Source Discovery

Before starting the worker pool:

1. Resolve and validate the project root.
2. Walk the project directory once.
3. Skip symlinks and excluded directories.
4. Select regular files ending in `.js`, `.jsx`, `.ts`, or `.tsx`.
5. Normalize each selected path relative to the project root.
6. Sort the paths for deterministic job seeding.
7. Build a read-only file index used by every resolver.
8. Submit one `analyze_file` job per indexed file.

Excluded directories:

```text
.git
node_modules
dist
build
coverage
```

The scanner must not read files outside the validated project root.

## Import Extraction

The initial extractor recognizes string-literal module specifiers from:

```ts
import value from "./value";
import "./setup";
export { value } from "./value";
export * from "./value";
const value = require("./value");
const value = import("./value");
```

Only specifiers beginning with `./` or `../` proceed to local resolution.
Bare package imports, URL imports, computed expressions, and non-literal
dynamic imports are ignored.

Extraction must avoid matching import-like text inside comments or unrelated
string literals. A focused tokenizer or parser should be used instead of a
repository-wide regular expression.

## Local Import Resolution

Resolve each relative specifier from the importing file's directory against
the prebuilt file index. Try candidates in this fixed order:

1. The exact referenced path when it includes a supported extension.
2. `<specifier>.ts`
3. `<specifier>.tsx`
4. `<specifier>.js`
5. `<specifier>.jsx`
6. `<specifier>/index.ts`
7. `<specifier>/index.tsx`
8. `<specifier>/index.js`
9. `<specifier>/index.jsx`

Resolution must normalize `.` and `..` segments and reject candidates outside
the project root. When no candidate exists, record one unresolved-import
diagnostic and do not create an edge.

## Graph Model and Store

Add file-specific types rather than reusing package coordinates:

```go
type FileNode struct {
    Path string `json:"path"`
}

type FileEdge struct {
    From string `json:"from"`
    To   string `json:"to"`
}

type Diagnostic struct {
    Path    string `json:"path"`
    Import  string `json:"import,omitempty"`
    Message string `json:"message"`
}
```

The in-memory `FileGraphStore` must be safe for concurrent worker writes. It
must deduplicate nodes by path and edges by the complete `(from, to)` pair.

All discovered files are inserted as nodes before jobs start. Workers add
resolved edges and diagnostics as they process imports.

This deliberately matches the current auditor's in-memory result-storage
boundary. Durable file-graph result storage is not part of this feature.

## Handler Responsibilities

`FileGraphHandler.Handle` performs only these steps:

1. Validate the job type and decode its payload.
2. Revalidate that the relative path stays under the project root.
3. Read the source file with a fixed maximum size.
4. Extract supported import specifiers.
5. Resolve each relative import through the immutable file index.
6. Add resolved edges and unresolved-import diagnostics to the graph store.
7. Return no child jobs.

Transient file-read failures return an error so the existing retry and DLQ
behavior applies. Unsupported syntax or extraction failures are recorded as
diagnostics and return success so deterministic source problems are not retried
five times.

## JSON Output Contract

Write the report only after the worker pool finishes and shuts down
successfully:

```json
{
  "root": "personal-portfolio",
  "nodes": [
    { "path": "src/App.tsx" },
    { "path": "src/components/Button.tsx" }
  ],
  "edges": [
    {
      "from": "src/App.tsx",
      "to": "src/components/Button.tsx"
    }
  ],
  "diagnostics": []
}
```

Before encoding:

- sort nodes by path;
- sort edges by `from`, then `to`;
- sort diagnostics by path, import, then message;
- emit exact duplicate edges once;
- use normalized repository-relative paths only;
- write indented UTF-8 JSON ending with a newline.

## Component Changes

```text
cmd/auditor/
├── main.go                         <- parse and run --analysis files
└── main_test.go                    <- file-mode CLI tests
internal/filegraph/
├── discovery.go                    <- source-file discovery and index creation
├── discovery_test.go
├── extractor.go                    <- supported JavaScript/TypeScript imports
├── extractor_test.go
├── resolver.go                     <- deterministic local path resolution
├── resolver_test.go
├── handler.go                      <- analyze_file job handler
├── handler_test.go
├── store.go                        <- concurrent graph result storage
├── store_test.go
├── report.go                       <- deterministic JSON generation
└── report_test.go
```

No changes are planned for:

```text
internal/job
internal/queue
internal/worker
internal/dlq
internal/store
db/migrations
web
```

## Testing Plan

### Unit tests

| Area | Required cases |
|---|---|
| Discovery | Supported extensions, ignored directories, sorted paths, symlink exclusion |
| Extraction | Import, side-effect import, re-export, `require`, dynamic import, ignored bare imports, ignored comments |
| Resolution | Exact extension, extension inference, index files, parent paths, missing targets, root escape rejection |
| Store | Concurrent writes, node deduplication, edge deduplication, stable snapshots |
| Handler | Valid file, multiple imports, unresolved import, invalid payload, wrong job type, oversized file |
| Report | Empty graph, deterministic ordering, diagnostics, normalized paths, trailing newline |
| CLI | Default package mode unchanged, valid file mode, non-directory input, missing output, GitHub rejection |

### Integration test

Create a temporary local fixture containing a small JavaScript/TypeScript graph
with a cycle, one disconnected file, and one unresolved import. Run file mode
through the real queue and worker pool, then verify the JSON nodes, edges, and
diagnostics. The test must not use the network.

## Implementation Sequence

1. Add file graph types, concurrent store, and deterministic JSON renderer.
2. Add source discovery and immutable file index construction.
3. Add JavaScript/TypeScript import extraction and local resolution.
4. Add `FileGraphHandler` using the existing `job.Handler` contract.
5. Add the explicit CLI analysis mode and seed one job per source file.
6. Add unit and integration coverage.
7. Run formatting, unit tests, race-sensitive tests, and the existing full Go
   test suite to confirm package auditing remains unchanged.

## Acceptance Criteria

- Existing package-audit commands behave exactly as before.
- File mode accepts one local JavaScript/TypeScript project directory.
- Every supported source file appears exactly once in the output.
- Every resolvable relative import produces one correctly directed edge.
- External imports do not produce file edges.
- Missing local targets produce diagnostics rather than fabricated nodes.
- Cycles and disconnected files complete without hanging.
- File jobs run through the existing queue and bounded worker pool.
- The queue, worker, retry, DLQ, storage interfaces, database schema, UI, and
  deployment remain unchanged.
- Identical project contents produce byte-identical JSON output.
