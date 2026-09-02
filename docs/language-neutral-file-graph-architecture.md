# Language-Neutral File Graph Architecture

## Status

Proposed architecture. This document defines a future design boundary only;
it does not implement or reorganize the current file-graph code.

## Objective

Evolve the file dependency graph into a language-neutral engine that can
analyze polyglot repositories through small language-specific analyzers.

The engine must not attempt to parse every language through one universal
parser. Each analyzer owns the import syntax and resolution rules for its
language, while the shared engine owns discovery, queue orchestration, graph
storage, deterministic reporting, and the UI-facing output contract.

## Architecture

```text
Repository source
      |
      v
File discovery and language detection
      |
      v
Repository-wide immutable file index
      |
      v
Analyzer registry
  +---+------------+----------------+
  |                |                |
  v                v                v
JavaScript/TS    Python      Future analyzers
                              Go, Java, C#, Rust...
  |                |                |
  +----------------+----------------+
                   |
                   v
Language-specific reference resolution
                   |
                   v
Normalized file graph
                   |
                   v
Deterministic JSON output / UI
```

## Proposed Package Structure

```text
internal/filegraph/
├── graph.go                 # language-neutral nodes and edges
├── discovery.go             # file discovery and language detection
├── index.go                 # immutable repository-wide file index
├── handler.go               # queue-job orchestration
├── report.go                # deterministic output
├── diagnostics.go           # normalized analysis diagnostics
└── analyzer/
    ├── analyzer.go          # analyzer contract and registry
    ├── javascript/
    │   ├── extractor.go
    │   └── resolver.go
    ├── python/
    │   ├── extractor.go
    │   └── resolver.go
    └── go/
        ├── extractor.go
        └── resolver.go
```

This is a target structure, not an instruction to move the current files
immediately. The extraction should happen only when another language is being
added or when a separate refactor is explicitly approved.

## Core Analyzer Contract

Each language analyzer implements one narrow interface:

```go
type Analyzer interface {
    Supports(path string) bool
    Analyze(ctx context.Context, file FileContext) (Result, error)
}
```

The shared input provides only repository-scoped data:

```go
type FileContext struct {
    Root  string
    Path  string
    Index Index
}
```

The analyzer returns normalized results rather than modifying the graph store
directly:

```go
type Result struct {
    Dependencies []FileDependency
    Diagnostics  []Diagnostic
}
```

Analyzer dependencies use a language-independent representation:

```go
type FileDependency struct {
    Target     string
    Kind       string // import, include, require, reference
    Confidence string // exact, inferred
}
```

This boundary keeps language parsing and resolution testable without the
queue, worker pool, report writer, or UI.

## Analyzer Registry

The handler selects an analyzer through an immutable registry constructed
before workers start:

```go
type Registry interface {
    AnalyzerFor(path string) (Analyzer, bool)
}
```

Registry rules:

- analyzer selection is deterministic;
- a path is assigned to at most one analyzer;
- unsupported files are skipped safely;
- unsupported languages can be summarized in diagnostics or metadata;
- analyzers cannot register themselves dynamically from repository code;
- repository source is never executed to discover an analyzer.

## Shared Repository Index

Discovery walks the repository once and builds one immutable index before file
jobs are submitted. Every analyzer resolves references against this same
repository snapshot.

The index should provide normalized repository-relative paths and bounded
lookups without embedding language-specific package rules. Language analyzers
may build read-only derived indexes, such as Python module names or Java
package declarations, before workers start when required.

The shared index prevents every file job from repeatedly walking the
repository and ensures all workers analyze the same source snapshot.

## Normalized Graph Model

Nodes and edges remain independent of programming language:

```go
type Node struct {
    Path     string `json:"path"`
    Language string `json:"language,omitempty"`
}

type Edge struct {
    From       string `json:"from"`
    To         string `json:"to"`
    Kind       string `json:"kind,omitempty"`
    Confidence string `json:"confidence,omitempty"`
}
```

Suggested normalized edge values:

| Field | Examples | Meaning |
|---|---|---|
| `kind` | `import`, `include`, `require`, `reference` | Source-language relationship normalized for consumers |
| `confidence` | `exact`, `inferred` | Whether resolution was direct or heuristic |

Unresolved references do not create invented target nodes or edges. They are
reported as diagnostics:

```go
type Diagnostic struct {
    Path      string `json:"path"`
    Reference string `json:"reference,omitempty"`
    Message   string `json:"message"`
}
```

## Output Contract

Introduce an explicit schema version before adding optional graph metadata:

```json
{
  "schemaVersion": 1,
  "root": "repository",
  "nodes": [],
  "edges": [],
  "diagnostics": []
}
```

The report layer owns deterministic ordering:

- nodes sorted by normalized path;
- edges sorted by `from`, `to`, `kind`, and `confidence`;
- diagnostics sorted by path, reference, and message;
- exact duplicate nodes and edges emitted once;
- repository-relative slash-separated paths used as stable identities.

New optional fields require a schema compatibility decision. Existing fields
must not silently change meaning.

## Queue and Worker Integration

Continue using one `analyze_file` job per discovered supported file. The
shared handler performs only orchestration:

1. Validate and decode the file job.
2. Select the analyzer from the registry.
3. Pass the repository-scoped file context to the analyzer.
4. Add normalized dependencies and diagnostics to the shared graph store.
5. Return no child jobs.

The analyzer boundary does not require changes to the generic job, queue, or
worker interfaces.

Local and temporary GitHub analysis should continue using the in-memory queue.
A durable file-analysis queue would require durable source snapshots and is a
separate architecture decision.

## Polyglot Repository Behavior

One repository may contain several supported languages:

```text
frontend/App.tsx       -> JavaScript/TypeScript analyzer
backend/main.py        -> Python analyzer
service/server.go      -> Go analyzer
android/App.java       -> Java analyzer
```

All analyzers write to the same normalized file graph. Cross-language edges
are recorded only when an analyzer can resolve them from explicit repository
evidence. Unsupported files remain absent from the graph rather than being
misclassified.

Supporting every repository means graceful partial analysis, not guaranteed
perfect static resolution. Reflection, computed imports, generated code,
macros, custom build systems, runtime plugin loading, and missing generated
artifacts may remain unresolved and should be represented honestly through
diagnostics.

## Separation From Package Dependencies

Package and file dependency graphs remain separate models:

| Concern | Package graph | File graph |
|---|---|---|
| Node identity | Package plus version | Repository-relative file path |
| Edge meaning | Package requires package | Source file references source file |
| Resolver | Package registry or manifest rules | Language-specific source rules |
| Metadata | Version, license, policy verdict | Language, relationship kind, confidence |

The UI may display both as separate modes or layers later, but file edges must
not be inserted into package stores and package edges must not be presented as
file imports.

## Adding a Language

A future language addition should be limited to:

1. Add file-extension or file-signature detection.
2. Implement the analyzer contract.
3. Extract supported static dependency references.
4. Resolve those references through the repository index and any bounded
   language-specific derived index.
5. Register the analyzer explicitly.
6. Add extractor, resolver, registry, and polyglot integration tests.
7. Document unsupported dynamic or build-system behavior.

It should not require changes to the graph store, queue contract, worker
contract, GitHub acquisition, or UI graph mapping.

## Migration Boundary

The current JavaScript/TypeScript and Python behavior should remain unchanged
until an analyzer-registry refactor is explicitly implemented. A future
migration should proceed incrementally:

1. Define the normalized analyzer contract and registry.
2. Move the existing JavaScript/TypeScript logic behind one adapter.
3. Move the existing Python logic behind one adapter.
4. Prove output compatibility with golden JSON tests.
5. Add the next language only after compatibility is established.

Do not combine this migration with UI redesign, package-graph unification,
database persistence, incremental analysis, or queue redesign.

## Out of Scope

- Implementing or moving current source files.
- Adding Go, Java, C#, Rust, or another analyzer now.
- UI changes or combined graph presentation.
- Package dependency or auditor changes.
- Queue, worker, retry, DLQ, or database changes.
- Durable repository snapshots or persistent file-graph storage.
- Runtime execution of repository source.
- Call graphs, symbol graphs, data-flow analysis, or architecture enforcement.
- Guaranteed resolution of dynamic or generated dependencies.

## Acceptance Criteria for the Future Architecture

- The shared engine contains no language-specific parsing rules.
- Every supported language is selected through an explicit analyzer registry.
- One immutable repository index is reused across all file jobs.
- Analyzer results normalize into the same node, edge, and diagnostic model.
- Polyglot repositories can produce one deterministic file graph.
- Unsupported languages and unresolved references fail gracefully.
- Existing JavaScript/TypeScript and Python output remains compatible through
  the migration.
- Package dependencies remain separate from file dependencies.
- Queue, worker, GitHub acquisition, and UI contracts do not require
  language-specific changes.
