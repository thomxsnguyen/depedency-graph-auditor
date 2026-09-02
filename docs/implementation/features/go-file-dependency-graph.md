# Go File Dependency Graph — Feature Plan

## Status

Implemented. The completed Go file analyzer follows the boundaries below.

## Objective

Add a Go analyzer to the existing language-neutral file-graph registry so that
file-analysis mode can discover `.go` files and map imports of repository-local
Go packages to their source files.

The feature extends this existing command:

```bash
go run ./cmd/auditor \
  --analysis files \
  --output file-graph.json \
  ./go-project
```

It must also work with the existing public GitHub repository input:

```bash
go run ./cmd/auditor \
  --analysis files \
  --output file-graph.json \
  https://github.com/example/go-project
```

## Go Dependency Semantics

Go source files import packages, not individual files. A local package is
compiled from the eligible `.go` files in its directory.

For:

```go
// cmd/server/main.go
import "example.com/project/internal/config"
```

and:

```text
internal/config/load.go
internal/config/types.go
```

the normalized file graph records:

```text
cmd/server/main.go -> internal/config/load.go
cmd/server/main.go -> internal/config/types.go
```

This means the importing file depends on the local package compiled from those
files. The analyzer must not select one arbitrary representative file.

Files in the same Go package do not receive implicit edges merely because they
share a directory. This feature records explicit import relationships only.

## Fixed Scope

### Included

- Files ending in `.go`.
- One root `go.mod` file used to identify the repository module path.
- Standard Go import declarations parsed with the Go standard library parser.
- Single imports and grouped import blocks.
- Aliased, blank, and dot imports.
- Repository-local imports matching the root module path.
- Mapping one local package import to the package's indexed `.go` files.
- Existing one-`analyze_file`-job-per-file processing.
- Existing local-directory and public-GitHub file-analysis inputs.
- Deterministic nodes, edges, and diagnostics in the current versioned JSON
  schema.
- Unit, compatibility, polyglot, and orchestration tests.

### Excluded

- Changes to `go.mod` package dependency auditing.
- Downloading external Go modules.
- Edges to standard-library or third-party packages.
- Executing `go list`, `go env`, `go test`, or repository code.
- Resolving `replace` directives to directories outside the repository.
- Go workspaces defined by `go.work`.
- Nested Go modules or multiple `go.mod` files.
- Build-tag, `GOOS`, `GOARCH`, or cgo-specific file selection.
- Generated-file analysis beyond treating generated `.go` files as ordinary
  indexed source files.
- Symbol, function-call, type-reference, interface, or data-flow graphs.
- UI changes or package/file graph combination.
- Queue, worker, retry, DLQ, shutdown, database, or GitHub acquisition changes.
- Adding another source language.

## Existing Architecture Integration

Add one analyzer behind the current language-neutral contract:

```text
Repository discovery
        |
        v
Immutable repository index
        |
        v
Analyzer registry
        |
        +--> JavaScript/TypeScript analyzer
        +--> Python analyzer
        +--> Go analyzer
                  |
                  v
          local Go package resolution
                  |
                  v
        normalized file dependencies
```

The Go analyzer implements the existing interface:

```go
type Analyzer interface {
    Supports(path string) bool
    Analyze(ctx context.Context, file FileContext) (Result, error)
}
```

Register it explicitly in the CLI composition root:

```go
registry, err := analyzer.NewRegistry(
    javascript.New(),
    python.New(),
    golang.New(moduleIndex),
)
```

Do not add extension switches or Go-specific parsing to the shared handler.

## Proposed Files

```text
internal/filegraph/analyzer/golang/
├── analyzer.go
├── analyzer_test.go
├── extractor.go
├── extractor_test.go
├── module_index.go
├── module_index_test.go
├── resolver.go
└── resolver_test.go
```

Scoped existing-file changes:

```text
cmd/auditor/main.go                  # construct and register Go analyzer
cmd/auditor/main_test.go             # file-analysis regression coverage
internal/filegraph/discovery.go      # discover .go and exclude vendor
internal/filegraph/discovery_test.go
internal/filegraph/compatibility_test.go
```

Do not modify:

```text
internal/auditor/
internal/depfile/
internal/dlq/
internal/github/
internal/gomod/
internal/job/
internal/pypi/
internal/queue/
internal/store/
internal/worker/
db/migrations/
web/
```

## Discovery

Extend shared discovery to include regular `.go` files. Preserve the existing
JavaScript/TypeScript and Python extensions.

During the existing repository walk, also collect repository-relative
`go.mod` paths as discovery metadata. Manifest paths do not become graph nodes
or file-analysis jobs. This metadata allows root and nested module boundaries
to be identified without a second repository traversal.

Add `vendor` to the excluded directory names so vendored dependency source
does not become repository file nodes. Continue excluding `.git`, virtual
environments, package caches, build output, `node_modules`, and the existing
excluded directories.

Include `_test.go` files as source nodes because tests can contain meaningful
local imports. Test files are analyzed as importers but are not included as
target files for a package imported by non-test source.

## Root Module Identification

Before file jobs start:

1. Require `go.mod` in the discovery metadata at the repository root.
2. Parse its `module` directive without executing the Go toolchain.
3. Reject an empty or malformed module path with a clear diagnostic.
4. Build one read-only Go module index shared by all Go analyzer instances.

If no root `go.mod` exists, `.go` files may remain visible as nodes, but local
package imports cannot be resolved reliably. Record one repository-level or
file-level diagnostic and do not invent edges.

If nested `go.mod` files are discovered, report that nested modules are not
supported by this initial feature. Do not merge their module namespaces.

## Import Extraction

Use the Go standard library packages `go/parser`, `go/ast`, and `go/token`.
Parse imports only; do not type-check, compile, or execute source.

Recognize:

```go
import "example.com/project/internal/config"

import (
    "example.com/project/internal/database"
    alias "example.com/project/internal/service"
    _ "example.com/project/internal/register"
    . "example.com/project/internal/helpers"
)
```

Return normalized import paths without quote characters. Parser errors become
deterministic diagnostics and must not consume queue retry attempts.

## Module Index

Build the Go-specific derived index once before workers start:

```go
type ModuleIndex struct {
    ModulePath string
    Packages   map[string][]string
}
```

`Packages` maps a repository-relative package directory to its sorted source
files:

```text
internal/config ->
  internal/config/load.go
  internal/config/types.go
```

Index rules:

- include indexed `.go` files beneath the root module;
- group files by repository-relative directory;
- sort and deduplicate every file list;
- exclude `vendor`;
- keep `_test.go` files distinguishable from ordinary package files;
- expose immutable copies or read-only lookup methods;
- never walk the repository a second time.

Because build constraints are outside this initial scope, the module index is
target-independent and may include mutually exclusive platform files. This is
an intentional conservative dependency view and must be documented in the
result diagnostics or command output rather than presented as a specific
`GOOS`/`GOARCH` build graph.

Package declarations may be parsed to validate a directory, but this feature
does not build a type index.

## Local Import Resolution

An import is repository-local when it equals the root module path or begins
with the root module path followed by `/`.

For module path:

```text
example.com/project
```

resolve:

```text
example.com/project/internal/config
```

to repository directory:

```text
internal/config
```

Resolution rules:

1. Ignore standard-library and external module imports.
2. Remove the exact root module prefix from a local import.
3. Normalize the remaining repository-relative directory.
4. Reject absolute paths and repository-root escapes.
5. Look up the package directory in the immutable module index.
6. Create one dependency for each eligible target `.go` file.
7. If the local package directory or eligible files are missing, return an
   unresolved-local-import diagnostic.

For a normal `.go` importer, target ordinary `.go` files and exclude
`_test.go`. For a `_test.go` importer, also exclude target `_test.go` files;
tests depend on the package implementation, not another package's tests.

An import of the root module maps to ordinary `.go` files located at the
repository root.

## Analyzer Result

Each resolved target becomes a normalized dependency:

```go
analyzer.Dependency{
    Target:     "internal/config/load.go",
    Kind:       "import",
    Confidence: "exact",
}
```

External imports produce neither edges nor diagnostics. Missing imports under
the known root module path produce:

```go
analyzer.Diagnostic{
    Reference: "example.com/project/internal/missing",
    Message:   "unresolved local import",
}
```

The shared handler continues mapping analyzer results into the existing graph
store. No Go-specific fields are added to the handler or external JSON shape.

## Queue Behavior

- Submit one existing `analyze_file` job for every discovered `.go` file.
- Use the existing in-memory queue and bounded worker pool.
- Return no child jobs.
- Treat file-read failures as retryable errors.
- Treat parser failures and unresolved local imports as deterministic
  diagnostics.
- Continue allowing file analysis without `DATABASE_URL`.

No job type, queue option, worker option, retry policy, or shutdown behavior
changes are required.

## Output Contract

Use the existing language-neutral file graph schema:

```json
{
  "schemaVersion": 1,
  "root": "go-project",
  "nodes": [
    { "path": "cmd/server/main.go" },
    { "path": "internal/config/load.go" }
  ],
  "edges": [
    {
      "from": "cmd/server/main.go",
      "to": "internal/config/load.go"
    }
  ],
  "diagnostics": []
}
```

Preserve the existing deterministic ordering and external field meanings. Do
not introduce package nodes, module nodes, language metadata, or UI-specific
properties in this feature.

## Testing Plan

### Extraction tests

- single import;
- grouped imports;
- alias import;
- blank import;
- dot import;
- standard-library import;
- malformed Go syntax;
- import-like text in comments and string literals.

### Module-index tests

- valid root module path;
- missing root `go.mod`;
- malformed or missing `module` directive;
- root package files;
- nested package directories;
- deterministic file ordering;
- `_test.go` classification;
- `vendor` exclusion;
- nested module diagnostic.

### Resolution tests

- root module import;
- nested local package import;
- multiple files in one imported package;
- external module exclusion;
- standard-library exclusion;
- unresolved known-local package;
- module-prefix boundary, such as rejecting `example.com/project-other`;
- repository-root escape rejection;
- target `_test.go` exclusion.

### Analyzer tests

- one import produces all eligible target-file dependencies;
- multiple imports deduplicate through the graph store;
- parser failures return diagnostics without errors;
- source read failures return errors;
- unsupported extensions are rejected by `Supports`;
- source code is never executed.

### Polyglot regression test

Extend the existing compatibility fixture with one root Go module containing
local package imports. Verify JavaScript/TypeScript, Python, and Go analyzers
produce one deterministic graph without changing existing JS/TS or Python
edges and diagnostics.

### Full regression

```bash
gofmt -w <changed-go-files>
go test ./internal/filegraph/...
go test -race ./internal/filegraph/... ./cmd/auditor
go test ./...
go vet ./...
git diff --check
```

## Implementation Sequence

1. Extend discovery for `.go` and exclude `vendor`.
2. Add Go import extraction using the standard library parser.
3. Add root `go.mod` module-path parsing.
4. Build the immutable Go module index from discovered paths.
5. Add local module-import resolution.
6. Implement the Go analyzer contract.
7. Register the Go analyzer in file-analysis CLI composition.
8. Extend compatibility and polyglot tests.
9. Run race, complete-suite, vet, and scope validation.

## Acceptance Criteria

- `.go` files are discovered and processed through existing file-analysis
  jobs.
- Go imports are parsed without executing the repository or Go toolchain.
- Imports under the root module path resolve to all eligible source files in
  the local target package.
- Standard-library and external module imports produce no file edges.
- Missing known-local packages produce deterministic diagnostics.
- Vendored and target `_test.go` files do not become imported-package edges.
- JavaScript/TypeScript and Python file-analysis behavior remains unchanged.
- Package dependency analysis from `go.mod` remains unchanged.
- Local and public-GitHub file analysis continue to use the existing flow.
- Output remains deterministic and uses `schemaVersion: 1`.
- No UI, database, GitHub acquisition, queue, worker, or additional-language
  changes are included.
