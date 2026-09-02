# Go File Dependency Graph — Implementation Plan

## Status

Implemented. The completed implementation follows the sequence and boundaries
defined below.

## Source Feature

This plan implements the behavior defined in
[`go-file-dependency-graph.md`](go-file-dependency-graph.md).

## Objective

Add one Go analyzer to the existing language-neutral file-graph registry. The
analyzer will parse imports from `.go` files and resolve repository-local Go
package imports to the ordinary `.go` files that compile into the target
package.

The implementation must preserve the current JavaScript/TypeScript and Python
file graphs, public GitHub input, in-memory queue behavior, and versioned JSON
shape.

## Fixed Implementation Scope

### Included

- `.go` source discovery.
- `vendor` directory exclusion.
- Root and nested `go.mod` discovery metadata collected during the existing
  repository walk.
- Root module-path parsing with the existing `golang.org/x/mod/modfile`
  dependency.
- Go import extraction with `go/parser` in imports-only mode.
- An immutable Go module/package index derived from discovered paths.
- Local module-import resolution to all ordinary target `.go` files.
- Explicit registration of one Go analyzer.
- Deterministic diagnostics for missing root modules, nested modules, malformed
  Go source, and unresolved known-local imports.
- Existing file-analysis queue orchestration and JSON reporting.
- Unit, compatibility, polyglot, CLI, race, and regression tests.

### Excluded

- Go package-auditor or `go.mod` dependency-selection changes.
- External module downloads or resolution.
- `go list`, `go env`, compilation, tests, or repository-code execution.
- `go.work`, multiple active modules, or nested-module analysis.
- `replace` directives pointing outside the repository.
- Build-tag, platform, architecture, or cgo evaluation.
- Function, symbol, type, interface, call, or data-flow analysis.
- UI changes.
- Queue, worker, retry, DLQ, shutdown, database, or GitHub acquisition changes.
- Any language other than Go.

## Dependency Direction

Preserve the existing package direction:

```text
cmd/auditor
    |
    +--> filegraph
    +--> filegraph/analyzer
    +--> filegraph/analyzer/javascript
    +--> filegraph/analyzer/python
    +--> filegraph/analyzer/golang

filegraph/analyzer/golang --> filegraph/analyzer
filegraph/analyzer/golang --> golang.org/x/mod/modfile
```

The shared handler must not import the concrete Go analyzer package. The CLI
remains the composition root that constructs and registers concrete analyzers.

## Files to Add

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

## Files to Modify

```text
cmd/auditor/main.go
cmd/auditor/main_test.go
internal/filegraph/discovery.go
internal/filegraph/discovery_test.go
internal/filegraph/compatibility_test.go
internal/filegraph/testdata/compatibility/expected.json
```

The compatibility fixture may add only the minimum Go files and root `go.mod`
needed to verify one polyglot graph.

## Files That Must Not Change

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

## Step 1 — Extend Single-Pass Discovery

Add a richer discovery result without breaking callers that only need source
paths:

```go
type Discovery struct {
    Paths     []string
    Index     RepositoryIndex
    GoModules []string
}

func DiscoverRepository(root string) (Discovery, error)
```

`DiscoverRepository` performs one walk and:

1. Preserves `.js`, `.jsx`, `.ts`, `.tsx`, and `.py` discovery.
2. Adds regular `.go` files to `Paths` and `Index`.
3. Adds repository-relative `go.mod` paths to `GoModules` only.
4. Does not add `go.mod` to graph nodes or file-analysis jobs.
5. Skips `vendor` alongside the existing excluded directories.
6. Sorts and deduplicates source and module paths.
7. Continues skipping symlinks.

Keep the existing function as a compatibility wrapper:

```go
func Discover(root string) ([]string, RepositoryIndex, error)
```

The wrapper delegates to `DiscoverRepository`. File-analysis orchestration
must call `DiscoverRepository` directly so it does not perform a second walk.

Required discovery tests:

- `.go` files are included;
- root and nested `go.mod` paths are metadata only;
- `vendor` is excluded;
- existing JS/TS and Python discovery is unchanged;
- paths and module manifests are deterministic;
- symlinks remain excluded.

## Step 2 — Build the Go Module Index

Add `internal/filegraph/analyzer/golang/module_index.go`.

The immutable model should expose behavior rather than mutable maps:

```go
type ModuleIndex struct {
    modulePath    string
    packageFiles map[string][]string
    nestedRoots  []string
}

func BuildModuleIndex(
    root string,
    index analyzer.Index,
    moduleFiles []string,
) (ModuleIndex, []analyzer.Diagnostic, error)

func (i ModuleIndex) ModulePath() string
func (i ModuleIndex) PackageFiles(directory string) []string
func (i ModuleIndex) Owns(path string) bool
```

Construction behavior:

1. Find `go.mod` exactly at the repository root.
2. Read it through bounded repository-local file access.
3. Parse the module path with `modfile.ModulePath`.
4. Treat a missing or empty module directive as one deterministic diagnostic
   associated with `go.mod`.
5. Identify every non-root `go.mod` as a nested module root.
6. Return one deterministic unsupported-nested-module diagnostic for each
   nested `go.mod`.
7. Group indexed `.go` paths by repository-relative directory.
8. Exclude paths beneath `vendor` and nested module roots.
9. Separate ordinary `.go` files from `_test.go` files.
10. Sort and deduplicate all stored paths.
11. Copy caller-owned slices so the resulting index is safe for concurrent
    reads.

I/O failures return errors. Missing or malformed repository configuration is
a deterministic diagnostic and does not enter queue retry behavior.

When the root module is unavailable, return a disabled but valid `ModuleIndex`.
The Go analyzer still claims `.go` files but returns no invented dependencies;
the preflight diagnostic explains why resolution was unavailable.

## Step 3 — Extract Go Imports

Add `internal/filegraph/analyzer/golang/extractor.go`:

```go
func ExtractImports(filename string, source []byte) ([]string, error)
```

Implementation:

1. Create a new `token.FileSet`.
2. Call `parser.ParseFile` with `parser.ImportsOnly`.
3. Read imports from the parsed AST.
4. Decode each `ImportSpec.Path.Value` with `strconv.Unquote`.
5. Return import paths in source order.

This supports ordinary, grouped, aliased, blank, and dot imports without
special cases because the import path is stored consistently in the AST.

Malformed source returns a parser error. The Go analyzer converts that error
to a deterministic diagnostic and returns success to the worker.

Tests must cover:

- single and grouped imports;
- aliases, `_`, and `.` imports;
- raw and quoted import strings;
- comments and unrelated string literals;
- malformed import declarations;
- stable source ordering.

## Step 4 — Resolve Local Package Imports

Add `internal/filegraph/analyzer/golang/resolver.go`:

```go
type Resolution struct {
    Targets []string
    Local   bool
}

func Resolve(index ModuleIndex, importPath string) Resolution
```

Resolution rules:

1. If the module path is unavailable, return no targets and `Local: false`.
2. Treat the exact module path as the repository-root package.
3. Treat `modulePath + "/"` as the only valid local-import prefix.
4. Reject similar prefixes such as `modulePath-other`.
5. Ignore standard-library and external module imports.
6. Remove the module prefix and normalize the remaining directory with
   slash-based repository paths.
7. Reject absolute paths and root escapes.
8. Return all ordinary `.go` files in the target package directory.
9. Never return `_test.go` targets.
10. If the import is local but the package has no eligible targets, return
    `Local: true` with no targets.

Returned targets must already be sorted and copied from the immutable module
index.

Tests must cover the root package, nested packages, multi-file packages,
external imports, prefix boundaries, unresolved local packages, nested-module
exclusion, and `_test.go` exclusion.

## Step 5 — Implement the Go Analyzer

Add `internal/filegraph/analyzer/golang/analyzer.go`:

```go
type Analyzer struct {
    modules ModuleIndex
}

func New(modules ModuleIndex) *Analyzer
func (a *Analyzer) Supports(path string) bool
func (a *Analyzer) Analyze(
    ctx context.Context,
    file analyzer.FileContext,
) (analyzer.Result, error)
```

`Supports` returns true only for `.go` files.

`Analyze` performs:

1. If the file is beneath an unsupported nested module, return an empty result;
   the preflight nested-module diagnostic already explains the exclusion.
2. Read the source through the existing bounded analyzer source reader.
3. Extract imports with the standard-library parser.
4. Convert parser errors into one deterministic diagnostic.
5. Resolve each import through the immutable module index.
6. Ignore external and standard-library imports.
7. Add every resolved target as an exact `import` dependency.
8. Add an unresolved-local-import diagnostic when a known-local package has no
   eligible files.
9. Return no child jobs and never execute repository code.

Normalized dependency:

```go
analyzer.Dependency{
    Target:     target,
    Kind:       "import",
    Confidence: "exact",
}
```

Normalized unresolved diagnostic:

```go
analyzer.Diagnostic{
    Reference: importPath,
    Message:   "unresolved local import",
}
```

The shared graph store remains responsible for deduplicating identical edges.

## Step 6 — Compose the Analyzer in File Mode

Update `executeFileAnalysis` only:

1. Call `filegraph.DiscoverRepository` once.
2. Add `Discovery.Paths` as graph nodes.
3. Build the Go module index from the same discovery result.
4. Add preflight module diagnostics to the existing graph store in
   deterministic order.
5. Register `golang.New(moduleIndex)` after the existing JavaScript/TypeScript
   and Python analyzers.
6. Submit the same one `analyze_file` job per discovered source path.

Target registry construction:

```go
registry, err := analyzer.NewRegistry(
    javascript.New(),
    python.New(),
    golang.New(moduleIndex),
)
```

Do not change package-analysis setup, GitHub download/extraction, the queue,
the worker pool, shutdown handling, or report writing.

## Step 7 — Preserve Output Compatibility

Continue emitting:

```json
{
  "schemaVersion": 1,
  "root": "repository",
  "nodes": [],
  "edges": [],
  "diagnostics": []
}
```

Do not add module nodes, package nodes, language fields, edge-kind fields, or
Go-specific metadata to the external schema.

Ordering remains owned by `filegraph.GenerateReport`:

- nodes by path;
- edges by `from`, then `to`;
- diagnostics by path, import, then message.

Build constraints are intentionally not evaluated. The Go analyzer therefore
produces a conservative, target-independent file graph and must not claim that
the output represents one specific `GOOS` or `GOARCH` build.

## Step 8 — Extend Compatibility Coverage

Add the minimum Go fixture to the existing polyglot compatibility repository:

```text
go.mod
cmd/server/main.go
internal/config/load.go
internal/config/types.go
internal/config/config_test.go
vendor/example.com/dependency/ignored.go
```

The fixture import is:

```go
import "example.com/compatibility/internal/config"
```

Expected additions:

- nodes for the non-vendored `.go` files, including `config_test.go`;
- edges from `cmd/server/main.go` to `load.go` and `types.go`;
- no edge to `config_test.go`;
- no node or edge for the vendored file;
- no changes to existing JavaScript/TypeScript or Python expectations;
- byte-identical JSON across repeated runs.

## Step 9 — Focused Tests

### Discovery

- `.go` inclusion;
- `vendor` exclusion;
- module metadata without graph nodes;
- root and nested module ordering;
- existing language regression.

### Module index

- valid root module;
- missing root `go.mod`;
- empty or malformed module directive;
- root and nested package directories;
- multiple ordinary files;
- `_test.go` separation;
- nested module exclusion and diagnostics;
- immutable returned slices;
- concurrent reads under the race detector.

### Resolver

- exact root-module import;
- nested local import;
- external and standard-library exclusion;
- module-prefix boundary;
- missing known-local package;
- target-test exclusion;
- deterministic multi-file targets.

### Analyzer

- supported `.go` extension;
- all static import forms;
- normalized exact dependencies;
- unresolved local diagnostics;
- malformed syntax as a non-retryable diagnostic;
- file-read failure as an error;
- disabled index behavior;
- unsupported nested-module file behavior.

### CLI orchestration

- local Go file graph;
- GitHub Go file graph through the existing injected archive client;
- file analysis with `DATABASE_URL` unset;
- deterministic report output;
- current JavaScript/TypeScript and Python output unchanged.

## Step 10 — Validation

Run:

```bash
gofmt -w <changed-go-files>
go test ./internal/filegraph/...
go test -race ./internal/filegraph/... ./cmd/auditor
go test ./...
go vet ./...
git diff --check
```

Then perform a changed-file audit. The implementation must not contain changes
under the directories listed in **Files That Must Not Change**.

## Implementation Sequence

1. Add single-pass discovery metadata and regression tests.
2. Add root module parsing and immutable module-index tests.
3. Add Go import extraction and tests.
4. Add local package resolution and tests.
5. Add the Go analyzer and tests.
6. Register it in file-analysis composition.
7. Extend the existing polyglot compatibility fixture.
8. Add local and GitHub orchestration regression tests.
9. Run focused, race, complete-suite, vet, formatting, and scope validation.

## Acceptance Criteria

- `.go` files use the existing `analyze_file` job path.
- Discovery performs one repository walk and records `go.mod` only as metadata.
- One immutable module index is shared across Go file jobs.
- Go imports are parsed without invoking the Go toolchain or executing source.
- Repository-local package imports resolve to every ordinary target `.go` file.
- `_test.go` files can be importers but never imported-package targets.
- Standard-library and external module imports produce no edges.
- Missing known-local packages produce deterministic diagnostics.
- Missing or malformed root modules and nested modules are reported without
  invented edges.
- Existing JavaScript/TypeScript and Python graphs remain compatible.
- Existing Go package dependency auditing remains unchanged.
- Local and public-GitHub file analysis remain available without PostgreSQL.
- Output remains deterministic `schemaVersion: 1` JSON.
- No UI, database, GitHub acquisition, queue, worker, additional-language, or
  package-auditor implementation is included.
