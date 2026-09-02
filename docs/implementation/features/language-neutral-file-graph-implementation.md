# Language-Neutral File Graph — Implementation Plan

## Status

Implemented. The completed migration follows the boundaries defined below.

## Source Architecture

This plan implements the design defined in
[`docs/language-neutral-file-graph-architecture.md`](../../language-neutral-file-graph-architecture.md).

## Objective

Refactor the existing JavaScript/TypeScript and Python file dependency
analysis behind a language-neutral analyzer registry while preserving current
behavior, queue usage, deterministic output, and GitHub repository support.

This implementation establishes the extension point for future languages. It
does not add another language.

## Fixed Scope

### Included

- A language-neutral analyzer contract.
- An explicit, immutable analyzer registry.
- One repository-wide immutable file index.
- JavaScript/TypeScript analyzer adapter using the existing extraction and
  resolution behavior.
- Python analyzer adapter using the existing extraction and resolution
  behavior.
- Language-neutral dependencies and diagnostics returned by analyzers.
- Existing one-`analyze_file`-job-per-file orchestration.
- Deterministic normalized graph output.
- A `schemaVersion` field added to the file-graph JSON contract.
- Compatibility, registry, analyzer, polyglot, and orchestration tests.

### Excluded

- Adding Go, Java, Kotlin, C#, Rust, or another language analyzer.
- Adding new JavaScript/TypeScript or Python syntax support.
- UI changes or combined graph presentation.
- Package dependency graph changes.
- GitHub download or archive-extraction changes.
- Queue, worker, retry, DLQ, or shutdown contract changes.
- Database migrations or durable file-graph storage.
- Durable repository snapshots.
- Watch mode, caching, or incremental analysis.
- Call graphs, symbol graphs, data-flow analysis, or architecture rules.
- Runtime execution of repository source.

## Compatibility Requirements

The refactor must preserve these current behaviors:

- `.js`, `.jsx`, `.ts`, `.tsx`, and `.py` discovery;
- existing excluded directories;
- existing JavaScript/TypeScript import forms and resolution order;
- existing Python absolute, relative, root, and `src/` resolution;
- external dependency exclusion;
- unresolved-local-import diagnostics;
- repository-relative slash-separated paths;
- deterministic node, edge, and diagnostic ordering;
- local-directory and public-GitHub file analysis;
- operation without `DATABASE_URL`;
- use of the current in-memory queue for file analysis.

The only planned JSON contract addition is:

```json
"schemaVersion": 1
```

Existing fields retain their names and meanings. Node and edge metadata may be
represented internally but must not be emitted in this implementation unless
explicitly included in the versioned schema and compatibility tests.

## Dependency Direction

Use dependency direction that avoids Go import cycles:

```text
cmd/auditor
    |
    +--> filegraph
    |      |
    |      +--> filegraph/analyzer
    |
    +--> filegraph/analyzer/javascript
    +--> filegraph/analyzer/python

filegraph/analyzer/javascript --> filegraph/analyzer
filegraph/analyzer/python     --> filegraph/analyzer
```

The `analyzer` package owns contracts and contains no dependency on the parent
`filegraph` package. Language packages depend only on the analyzer contracts
and standard library. The CLI composition root constructs the registry and
passes it to the file-graph handler.

## Target File Structure

```text
internal/filegraph/
├── types.go                         # graph output types
├── discovery.go                     # shared discovery and language detection
├── index.go                         # immutable repository index
├── handler.go                       # analyzer selection and graph writes
├── report.go                        # deterministic versioned output
├── store.go                         # concurrent normalized graph store
└── analyzer/
    ├── analyzer.go                  # contracts
    ├── registry.go                  # immutable analyzer registry
    ├── registry_test.go
    ├── javascript/
    │   ├── analyzer.go
    │   ├── extractor.go
    │   ├── resolver.go
    │   └── analyzer_test.go
    └── python/
        ├── analyzer.go
        ├── extractor.go
        ├── resolver.go
        └── analyzer_test.go
```

Tests that validate graph storage, reporting, discovery, and orchestration
remain in `internal/filegraph`. Existing extractor and resolver tests move
with their language implementation without changing their assertions.

## Step 1 — Lock Existing Behavior

Before moving implementation code, add compatibility fixtures covering:

1. One JavaScript/TypeScript repository fixture.
2. One Python root-layout fixture.
3. One Python `src/`-layout fixture.
4. One mixed JavaScript/TypeScript and Python fixture.
5. Existing unresolved local imports.
6. Existing external import exclusion.

Generate expected reports through the current handler and store them as test
fixtures. Normalize only temporary root paths; graph paths and ordering must
remain exact.

These tests provide the migration baseline. Do not change expected edges or
diagnostics to make the refactor pass.

## Step 2 — Define Analyzer Contracts

Add `internal/filegraph/analyzer/analyzer.go`:

```go
package analyzer

type Index interface {
    Has(path string) bool
    Paths() []string
}

type FileContext struct {
    Root  string
    Path  string
    Index Index
}

type Dependency struct {
    Target     string
    Kind       string
    Confidence string
}

type Diagnostic struct {
    Reference string
    Message   string
}

type Result struct {
    Dependencies []Dependency
    Diagnostics  []Diagnostic
}

type Analyzer interface {
    Supports(path string) bool
    Analyze(ctx context.Context, file FileContext) (Result, error)
}
```

Contract rules:

- `Supports` is deterministic and performs no I/O;
- `Analyze` reads only the validated file in `FileContext`;
- analyzers do not mutate the graph store;
- dependency targets are normalized repository-relative paths;
- unresolved references are diagnostics, not invented dependencies;
- deterministic source problems return diagnostics rather than retryable
  errors;
- transient file-read failures may return errors;
- analyzers never execute repository code.

## Step 3 — Add the Immutable Registry

Add `internal/filegraph/analyzer/registry.go`:

```go
type Registry struct {
    analyzers []Analyzer
}

func NewRegistry(analyzers ...Analyzer) (*Registry, error)
func (r *Registry) AnalyzerFor(path string) (Analyzer, bool, error)
```

Selection must return an ambiguity error when more than one analyzer claims
the same path. The registry is immutable after construction and safe for
concurrent reads.

Initial registration order is explicit in the CLI composition root:

```go
registry, err := analyzer.NewRegistry(
    javascript.New(),
    python.New(),
)
```

Do not use package `init` functions, runtime plugins, reflection, or repository
configuration to register analyzers.

## Step 4 — Introduce the Repository Index

Replace direct use of the mutable map alias with an immutable index type in
`internal/filegraph/index.go`:

```go
type RepositoryIndex struct {
    paths []string
    set   map[string]struct{}
}

func NewRepositoryIndex(paths []string) RepositoryIndex
func (i RepositoryIndex) Has(path string) bool
func (i RepositoryIndex) Paths() []string
```

Requirements:

- normalize and sort paths at construction;
- deduplicate paths;
- copy caller-owned slices and maps;
- return copies from `Paths`;
- allow concurrent read-only access;
- satisfy `analyzer.Index` without importing language packages.

Update discovery to return the stable path list and this index from the same
walk. Do not add a second repository traversal.

## Step 5 — Adapt JavaScript and TypeScript

Move the existing JavaScript/TypeScript extractor and resolver implementation
behind `analyzer/javascript.Analyzer`.

The adapter must preserve:

- `.js`, `.jsx`, `.ts`, and `.tsx` support;
- static imports and side-effect imports;
- re-exports;
- string-literal `require()`;
- string-literal dynamic `import()`;
- current relative-path resolution order;
- external package exclusion;
- current parsing and unresolved-import diagnostics.

Return normalized dependencies:

```go
analyzer.Dependency{
    Target:     resolvedPath,
    Kind:       "import",
    Confidence: "exact",
}
```

Do not add aliases, TypeScript path mappings, framework conventions, or new
syntax during this move.

Run the JavaScript/TypeScript compatibility tests before continuing.

## Step 6 — Adapt Python

Move the existing Python extractor and resolver implementation behind
`analyzer/python.Analyzer`.

The adapter must preserve:

- `.py` support;
- ordinary `import` statements;
- `from ... import ...` statements;
- aliases and parenthesized imported names;
- root-layout and `src/`-layout resolution;
- explicit relative imports;
- package `__init__.py` behavior;
- external module exclusion;
- unresolved-known-local diagnostics;
- current parser limitations.

Return normalized dependencies with `kind: "import"` and the existing exact
resolution semantics. Do not add namespace-package, dynamic-import, packaging,
or environment inspection behavior during this move.

Run the Python compatibility tests before continuing.

## Step 7 — Reduce the Handler to Orchestration

Update `filegraph.Handler` to receive an `analyzer.Registry` and
`RepositoryIndex`. Its `Handle` method performs only:

1. Validate the `analyze_file` job and repository-relative path.
2. Select an analyzer for the path.
3. Call the analyzer with the immutable file context.
4. Map returned dependencies to file-graph edges.
5. Map returned diagnostics to file-graph diagnostics using the job path.
6. Return no child jobs.

The handler must not contain extension switches or language-specific parsing
and resolution rules after migration.

If no analyzer supports a submitted path, return a deterministic diagnostic
or skip it according to the discovery contract. Do not retry unsupported
source files.

## Step 8 — Compose the Registry in the CLI

In file-analysis mode only:

1. Construct the JavaScript/TypeScript and Python analyzers.
2. Construct the immutable registry.
3. Discover files and build the repository index once.
4. Pass the registry and index to the file-graph handler.
5. Submit one job per discovered supported file through the existing in-memory
   queue.

Do not change package-audit construction or its PostgreSQL-backed queue.
Do not require `DATABASE_URL` for file analysis.

GitHub acquisition continues to produce a temporary repository root before
this flow starts and cleans it up after output is written.

## Step 9 — Version the JSON Report

Extend the file report type:

```go
type Report struct {
    SchemaVersion int          `json:"schemaVersion"`
    Root          string       `json:"root"`
    Nodes         []Node       `json:"nodes"`
    Edges         []Edge       `json:"edges"`
    Diagnostics   []Diagnostic `json:"diagnostics"`
}
```

Set `SchemaVersion` to `1` in one report-construction location. Preserve all
existing field meanings and deterministic ordering.

Do not emit `language`, `kind`, or `confidence` fields in version 1 unless the
current UI/output consumer is updated in a separately approved change.
The analyzer may use those fields internally while the report mapper preserves
the current external node and edge shape.

## Step 10 — Testing

### Contract and registry tests

- JavaScript/TypeScript paths select only the JavaScript analyzer.
- Python paths select only the Python analyzer.
- Unsupported paths select no analyzer.
- Ambiguous analyzer selection is rejected.
- Registry selection is safe under concurrent access.
- Analyzer results do not mutate index-owned data.

### Repository index tests

- Paths are normalized, deduplicated, and sorted.
- Constructor input cannot mutate the index.
- `Paths` results cannot mutate the index.
- Concurrent reads are race-free.

### Language compatibility tests

- Existing JavaScript/TypeScript extractor and resolver cases pass unchanged.
- Existing Python extractor and resolver cases pass unchanged.
- Existing handler diagnostics remain unchanged.
- Existing external import exclusions remain unchanged.

### Polyglot integration test

Create one fixture containing:

```text
frontend/App.tsx
frontend/Button.tsx
backend/app.py
backend/models.py
README.md
```

Verify:

- each supported file becomes one node;
- each file is handled by the correct analyzer;
- JavaScript/TypeScript edges remain within their resolved files;
- Python edges remain within their resolved files;
- the unsupported Markdown file is absent;
- nodes, edges, and diagnostics use one normalized graph;
- repeated runs produce byte-identical JSON.

### Orchestration regression tests

- local file analysis still uses the in-memory queue;
- GitHub file analysis still uses temporary extraction and cleanup;
- file analysis succeeds with `DATABASE_URL` unset;
- package analysis behavior remains unchanged;
- cancellation and shutdown behavior remains unchanged.

### Validation commands

```bash
gofmt -w <changed-go-files>
go test ./internal/filegraph/...
go test -race ./internal/filegraph/... ./cmd/auditor
go test ./...
go vet ./...
git diff --check
```

## Implementation Sequence

1. Add behavior-locking compatibility fixtures.
2. Add analyzer contracts and registry tests.
3. Add the immutable repository index.
4. Move JavaScript/TypeScript logic behind its analyzer adapter.
5. Move Python logic behind its analyzer adapter.
6. Convert the handler to registry-based orchestration.
7. Compose the two analyzers in file-analysis CLI setup.
8. Add `schemaVersion: 1` while preserving the existing graph shape.
9. Add polyglot and orchestration regression tests.
10. Run formatting, race tests, the complete Go suite, and a changed-file
    scope audit.

Do not begin a future language analyzer until this migration passes the
compatibility and deterministic-output tests.

## Expected Files Changed During Implementation

```text
cmd/auditor/main.go
cmd/auditor/main_test.go
internal/filegraph/discovery.go
internal/filegraph/discovery_test.go
internal/filegraph/handler.go
internal/filegraph/handler_test.go
internal/filegraph/report.go
internal/filegraph/report_test.go
internal/filegraph/types.go
internal/filegraph/index.go
internal/filegraph/index_test.go
internal/filegraph/analyzer/analyzer.go
internal/filegraph/analyzer/registry.go
internal/filegraph/analyzer/registry_test.go
internal/filegraph/analyzer/javascript/*
internal/filegraph/analyzer/python/*
internal/filegraph/testdata/*
```

Existing language extractor and resolver files may be moved into their target
language packages. No other production directories should change.

## Files That Must Not Change

```text
internal/auditor/
internal/depfile/
internal/dlq/
internal/github/
internal/job/
internal/pypi/
internal/queue/
internal/store/
internal/worker/
db/migrations/
web/
```

## Acceptance Criteria

- JavaScript/TypeScript and Python are selected through one explicit analyzer
  registry.
- The file-graph handler contains no language-specific extension switch,
  parser, or resolver logic.
- One immutable repository index is shared by all file jobs.
- Existing JavaScript/TypeScript and Python nodes, edges, and diagnostics are
  preserved.
- One polyglot repository produces one deterministic normalized graph.
- Unsupported source languages are skipped safely.
- JSON output includes `schemaVersion: 1` and preserves the existing graph
  field meanings.
- File analysis continues to use the existing in-memory queue and does not
  require PostgreSQL.
- Local and public-GitHub file analysis continue to work.
- Package dependency auditing remains unchanged.
- No UI, database, GitHub acquisition, queue, worker, or additional-language
  implementation is included.
