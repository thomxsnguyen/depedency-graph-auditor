# Feature Implementation — Go Dependency Manifest

## Status

Proposed feature. This document defines the implementation boundary for reading
one `go.mod` file and producing a deterministic Go module dependency graph. It
does not modify the current auditor.

## Goal

Allow the auditor to use one Go module manifest as its audit seed while
preserving the existing durable queue, bounded worker pool, retry, DLQ,
shutdown, graph, and Markdown report lifecycle.

Supported examples:

```bash
go run ./cmd/auditor \
  --ecosystem go \
  --output audit-report.md \
  ./go.mod
```

```bash
go run ./cmd/auditor \
  --ecosystem go \
  --manifest go.mod \
  --output audit-report.md \
  https://github.com/owner/repository
```

The feature reads module metadata. It must not run `go get`, `go mod tidy`,
`go list`, builds, tests, generators, or repository code.

## Scope

### In scope

- Preserve the existing npm and Python behavior.
- Accept one local file named `go.mod`.
- Accept one explicitly selected `go.mod` from a public GitHub repository
  through the existing GitHub manifest source.
- Parse the module path, Go version, and `require` directives.
- Retrieve dependency `go.mod` files through the public Go module proxy.
- Support canonical tagged versions, pseudo-versions, and `+incompatible`
  versions accepted by the Go module specification.
- Apply Minimal Version Selection (MVS) before mapping the final report graph.
- Respect module graph pruning and lazy loading rules associated with the
  relevant `go` directives.
- Represent Go module paths as package names and exact selected module versions
  as graph versions.
- Include both ordinary and `// indirect` module requirements in the module
  requirement graph.
- Reuse the existing durable queue for bounded, retryable module-metadata
  retrieval.
- Add deterministic parser, proxy, MVS, orchestration, and report tests without
  contacting public GitHub or the public Go proxy.

### Out of scope

- `go.work` workspaces or automatic multi-module discovery.
- `go.sum` parsing or checksum-database verification.
- Vendor directories and `vendor/modules.txt`.
- Package import graphs; this feature graphs modules and their requirements.
- Determining which dependencies are used by a particular build, test, tag,
  operating system, or architecture.
- Tool dependencies, source imports, build tags, and source-code analysis.
- Local filesystem replacements.
- Repository-relative or VCS replacements.
- Supporting root manifests containing `replace` or `exclude` directives in the
  first implementation.
- Private modules, `GOPRIVATE`, private proxies, credentials, or direct VCS
  fallback.
- Configurable proxy chains or arbitrary `GOPROXY` values.
- Downloading or extracting module ZIP archives.
- Executing the Go command or any module code.
- License discovery from source archives. Public proxy `.mod` metadata does not
  carry a canonical module license; Go modules use `UNKNOWN` for the existing
  policy field in this implementation.
- Vulnerability scanning, checksum verification, SBOM generation, or lockfile
  generation.
- Mixed-ecosystem or multi-manifest audits in one command.
- Changes to queue capacity, worker scheduling, retry policy, DLQ schema,
  Markdown graph layout, or report section structure.

## Standards

The implementation must follow the Go module specification rather than npm or
Python version behavior:

- Go Modules Reference: <https://go.dev/ref/mod>
- `go.mod` reference: <https://go.dev/doc/modules/gomod-ref>
- Module graph pruning: <https://go.dev/ref/mod#module-graph-pruning>
- Minimal Version Selection: <https://research.swtch.com/vgo-mvs>
- Go module proxy protocol: <https://go.dev/ref/mod#goproxy-protocol>
- `golang.org/x/mod/modfile`: <https://pkg.go.dev/golang.org/x/mod/modfile>
- `golang.org/x/mod/module`: <https://pkg.go.dev/golang.org/x/mod/module>
- `golang.org/x/mod/semver`: <https://pkg.go.dev/golang.org/x/mod/semver>

## CLI Contract

Extend the ecosystem selector:

```text
--ecosystem <npm|python|go>
```

Rules:

- `--ecosystem` continues to default to `npm`.
- Local Go input must have the basename `go.mod`.
- GitHub Go input requires `--manifest` and its selected basename must be
  `go.mod`.
- `--ref` retains its existing GitHub-only meaning.
- Exactly one positional local path or GitHub repository URL remains required.
- The auditor does not search a repository for nested Go modules.
- Python-only target options are rejected with `--ecosystem go`.
- Invalid ecosystem and manifest combinations fail before PostgreSQL is opened.

Example for a module in a repository subdirectory:

```bash
go run ./cmd/auditor \
  --ecosystem go \
  --manifest services/api/go.mod \
  --ref main \
  --output api-audit.md \
  https://github.com/owner/repository
```

## Manifest Selection

Manifest selection remains explicit and based on the logical basename:

| Ecosystem | Manifest | Parser |
|---|---|---|
| `npm` | `package.json` | Existing npm JSON parser |
| `python` | `pyproject.toml` | Existing standardized TOML parser |
| `python` | `requirements.txt` | Existing safe requirements parser |
| `go` | `go.mod` | Go module parser |

Content guessing is not allowed. A file with another basename must not be
silently interpreted as a Go manifest.

## Root `go.mod` Contract

Given:

```mod
module example.com/service

go 1.23

require (
    github.com/google/uuid v1.6.0
    golang.org/x/sync v0.16.0
)
```

the parser returns:

- root name: `example.com/service`;
- Go language version: `1.23`;
- exact initial requirements sorted by module path and version.

Rules:

- A valid, non-empty `module` directive is required.
- A valid `go` directive is required so graph-pruning behavior is explicit.
- Every `require` entry must contain a valid module path and canonical version.
- Single-line and parenthesized `require` forms are supported.
- `// indirect` requirements are retained; they are still module requirements.
- Duplicate or conflicting declarations follow the errors produced by the
  standards-aware parser and must not be silently merged.
- An empty `require` set preserves the existing no-work report behavior.
- A root `replace` or `exclude` directive returns a clear unsupported-feature
  error before PostgreSQL is opened.
- Root `retract` directives do not affect exact requirements and may be parsed
  but do not alter the audit graph.
- `toolchain`, `godebug`, and other directives that do not alter module
  requirements do not create graph nodes or edges.

Use `golang.org/x/mod/modfile` for parsing. Do not parse `go.mod` with regular
expressions or line splitting.

## Shared Manifest Boundary

The root parser may continue to return the shared manifest seed:

```go
type Manifest struct {
    Name         string
    Dependencies []Dependency
}
```

For Go:

- `Manifest.Name` is the root module path;
- `Dependency.Name` is a required module path;
- `Dependency.VersionRange` contains an exact canonical Go module version.

The field name remains unchanged for compatibility, but Go values are not
version ranges. They must never be passed to the npm semver or Python PEP 440
resolvers.

The Go parser also needs the root `go` directive for selection and pruning. It
may return a Go-specific parsed result internally and adapt its seed fields to
the shared manifest boundary at CLI orchestration.

## Go Module Proxy Contract

Production metadata comes only from the public Go module proxy:

```text
https://proxy.golang.org
```

For an exact module coordinate, fetch:

```http
GET /{escaped-module-path}/@v/{escaped-version}.mod
Accept: text/plain
```

The client must use `module.EscapePath` and `module.EscapeVersion`. It must not
construct proxy paths by applying ordinary URL escaping to raw module paths.

The response is the dependency module's `go.mod`. Parse dependency manifests
with `modfile.ParseLax`, consistent with Go's forward-compatibility behavior for
non-main modules. Only the dependency's module path, `go` directive, and
requirements affect traversal. `replace` and `exclude` directives in dependency
modules do not apply to the main module and must not redirect proxy requests.

Production defaults:

- base URL: `https://proxy.golang.org`;
- request timeout: 15 seconds;
- maximum `.mod` response: 5 MiB;
- no retries inside the proxy client;
- no fallback to direct VCS access;
- injectable base URL and HTTP client for deterministic tests.

The existing worker retry and DLQ behavior owns transient retries. The proxy
client must classify `404`/`410`, `429`, other non-success responses, timeout,
size, and decode failures clearly without including response bodies.

## Version Rules

Go dependency requirements normally identify exact versions. The feature must:

- validate versions with `golang.org/x/mod/module` and
  `golang.org/x/mod/semver`;
- enforce module-path major-version compatibility;
- support canonical releases such as `v1.6.0`;
- support pseudo-versions;
- support valid `+incompatible` versions;
- compare versions using Go module semantic-version ordering;
- reject npm ranges, Python constraints, branch names, tags without canonical
  versions, and malformed module coordinates.

Do not reuse `internal/semver`, whose behavior is designed for npm constraints.

## Minimal Version Selection

A recursive walk that renders every fetched coordinate is not a correct Go
build list. If two paths require different versions of the same module, MVS
selects the highest required version for that module path.

Example:

```text
root requires example.com/a v1.0.0
root requires example.com/b v1.0.0
example.com/a v1.0.0 requires example.com/shared v1.2.0
example.com/b v1.0.0 requires example.com/shared v1.5.0
```

The final graph contains `example.com/shared@v1.5.0`, not separate selected
`v1.2.0` and `v1.5.0` nodes.

To keep network retrieval concurrent while selection remains deterministic,
use a fetch-and-select workflow:

1. Parse the root requirements.
2. Compute the currently selected coordinate for each module path.
3. Queue metadata fetches only for selected coordinates not already cached.
4. Wait for that fetch round to reach a terminal state.
5. Add the fetched requirements to the module requirement graph.
6. Recompute the build list using MVS and the applicable graph-pruning rules.
7. Repeat until no selected coordinate changes and no selected metadata is
   missing.
8. Map only the final selected build list and its selected requirement edges
   into the existing package and edge stores.

Fetch completion order must not affect selection, node order, edge order, or
report bytes. A lower selected version may have been fetched during an earlier
round, but it must not remain as a node in the final report after promotion.

Do not import `cmd/go/internal/mvs`; internal Go packages are not a supported
library boundary. Implement the small required MVS boundary using public
`golang.org/x/mod` path/version utilities and cover it with table-driven tests.

## Graph-Pruning Rules

MVS and graph pruning are related but distinct. The implementation must use the
root and dependency `go` directives when deciding which requirement lists must
be loaded. It must not assume that recursively loading every `require` line is
equivalent to the build list for every Go version.

The selected implementation must follow the rules in the Go Modules Reference,
including the compatibility behavior for modules whose `go` version predates
module graph pruning. Tests must include graphs that mix pruning-aware and
pre-pruning modules.

This feature produces a module requirement graph, not an import graph. It does
not inspect packages to determine whether a module is used by a particular
build target.

## Final Graph Mapping

After selection stabilizes:

- the root node label is the root module path;
- each dependency node is `{module-path}@{selected-version}`;
- each edge points from a requiring module to the selected version of the
  required module path;
- root edges use the root module name and an empty root version, matching the
  current graph contract;
- duplicate edges are removed deterministically;
- nodes and edges are sorted before report rendering;
- discovered but unselected lower versions are excluded from the report.

If a module requires `shared@v1.2.0` but MVS selects `shared@v1.5.0`, the edge
points to `shared@v1.5.0`. The report therefore shows the selected module graph,
not a misleading mixture of requested and selected coordinates.

Both ordinary and `// indirect` requirements may produce edges. The current
graph schema has no edge-kind field, so the first implementation does not
visually distinguish them.

## License Metadata

The Go module proxy `.mod` endpoint does not include canonical license metadata.
Because downloading ZIP archives is out of scope, each selected Go module maps
to:

```text
License: UNKNOWN
```

`UNKNOWN` is passed through the existing policy checker without changing policy
semantics. The report must identify this metadata limitation. Adding safe module
archive inspection or another license source requires a separate feature.

## Queue Integration

The queue is used for one bounded, retryable external metadata request per
selected module coordinate. Jobs carry explicit Go coordinates and selection
round identity, for example:

```json
{
  "module_path": "golang.org/x/sync",
  "version": "v0.16.0",
  "round": 2
}
```

The Go metadata handler:

1. validates the payload;
2. fetches the exact `.mod` resource;
3. parses dependency requirements;
4. returns metadata to the Go selection coordinator.

It must not write directly to the final package or edge stores. Final graph
mapping happens only after MVS stabilizes, preventing stale lower versions from
leaking into the report.

The existing single-phase npm/Python handler remains unchanged. Go-specific
selection orchestration must not alter npm or Python traversal behavior.

## Execution Flow

```text
CLI ecosystem + go.mod
        ↓
local/GitHub ManifestSource
        ↓
strict root go.mod parser
        ↓
MVS selection coordinator
        ↓
queued public-proxy .mod fetch rounds
        ↓
stable selected build list
        ↓
existing package/edge stores
        ↓
existing terminal and Markdown reports
```

Detailed order:

1. Parse and validate CLI input.
2. Read or fetch exactly one root `go.mod`.
3. Strictly parse and validate the root module before opening PostgreSQL.
4. Reject unsupported root replacement or exclusion behavior.
5. Open PostgreSQL and create the existing queue infrastructure.
6. Run deterministic metadata-fetch and selection rounds.
7. Stop without generating a report if required jobs fail or shutdown fails.
8. Map the stable selected graph to existing stores.
9. Generate the existing terminal and optional Markdown report.

## Error Handling

Return clear errors for:

- an unsupported ecosystem or manifest combination;
- malformed `go.mod` syntax;
- missing or invalid `module` or `go` directives;
- invalid module paths or non-canonical versions;
- module-path major-version mismatches;
- unsupported root `replace` or `exclude` directives;
- missing module versions (`404` or `410`);
- proxy rate limits and other non-success responses;
- DNS, connection, TLS, timeout, response-size, and parse failures;
- a selection round that cannot obtain required metadata;
- queue shutdown or DLQ outcomes that make the graph incomplete.

Errors may identify public module paths, versions, and manifest locations. They
must not include authorization headers, environment contents, or response
bodies. No Markdown report may be generated from an incomplete selected graph.

## Security Constraints

- Fetch the root manifest only through the existing local-file or validated
  public GitHub source boundary.
- Fetch dependency metadata only from the fixed public Go proxy in production.
- Never honor proxy URLs, credentials, or replacement targets from manifest
  contents.
- Never fall back to VCS commands or repository cloning.
- Never execute the Go command, builds, tests, generators, or module code.
- Never download or extract module archives.
- Apply fixed HTTP timeouts and response-size limits.
- Parse module syntax with the standards-aware `x/mod` parser.
- Keep fetched `.mod` bytes in memory only for the current audit.

## Component Changes

```text
cmd/auditor/
├── main.go                         ← Go ecosystem selection and orchestration
└── main_test.go                    ← CLI/source and no-work tests
internal/depfile/
├── go.go                           ← strict root go.mod parser
└── go_test.go                      ← deterministic parser coverage
internal/gomod/
├── client.go                       ← public Go proxy .mod client
├── metadata.go                     ← lax dependency go.mod parsing
├── selection.go                    ← MVS and pruning coordinator
└── *_test.go                       ← proxy, MVS, pruning, and graph tests
```

Expected shared changes are limited to selecting Go orchestration and adapting
the final selected graph to existing report stores. Do not change npm or Python
resolvers, generic queue semantics, worker capacity, persistence schema, policy
rules, or Markdown layout.

## Testing Strategy

### CLI and source tests

| Test | Verification |
|---|---|
| Existing npm command | Continues to default to npm |
| Existing Python command | Python options and behavior remain unchanged |
| Local `go.mod` | Selects the Go parser |
| GitHub `go.mod` | Existing GitHub source supplies module bytes |
| Nested GitHub manifest | Explicit subdirectory `go.mod` is accepted |
| Missing GitHub manifest option | Go GitHub input is rejected clearly |
| Wrong basename | Fails before PostgreSQL opens |
| Python target option | Rejected with the Go ecosystem |
| Empty requirements | Preserves no-work behavior |

### Parser tests

| Test | Verification |
|---|---|
| Root metadata | Module path, Go version, and requirements are parsed |
| Deterministic order | Requirements are sorted by path and version |
| Indirect requirement | Retained as a module requirement |
| Canonical versions | Release, pseudo, and `+incompatible` cases parse |
| Invalid path/version | Returns a location-aware error |
| Major suffix mismatch | Rejected clearly |
| Missing module/go directive | Rejected before PostgreSQL opens |
| Root replacement | Local and remote replacements are rejected |
| Root exclusion | Rejected in the first implementation |
| Dependency-only directives | Non-main replace/exclude do not redirect fetching |

### Proxy-client tests

| Test | Verification |
|---|---|
| Escaped coordinate | Uppercase path/version components use module escaping |
| Exact `.mod` request | Correct proxy endpoint is requested |
| Dependency metadata | Go version and requirements are returned |
| Lax dependency parse | Unknown dependency-only directives remain compatible |
| Missing version | `404` and `410` are clear permanent failures |
| Rate limit | `429` remains retryable by the worker lifecycle |
| Timeout and size | Fixed safety limits are enforced |
| No fallback | Failure never invokes Git or another host |
| No public network | All HTTP behavior uses `httptest.Server` |

### MVS and pruning tests

| Test | Verification |
|---|---|
| Single version | Exact required coordinate is selected |
| Diamond graph | Highest shared-module version is selected |
| Version promotion | Lower fetched version is absent from the final graph |
| Pseudo-version comparison | Go semantic ordering is used |
| Major versions | `/v2` module path remains distinct and valid |
| Fixed point | Selection repeats until no selected coordinate changes |
| Fetch order | Different completion orders produce identical output |
| Fetch deduplication | A coordinate is fetched at most once per audit |
| Pruned graph | Modern Go directive follows pruning rules |
| Mixed Go versions | Pre-pruning dependency behavior is loaded correctly |
| Selected edges | Edges target final selected coordinates |

### Integration tests

| Test | Verification |
|---|---|
| Root mapping | Root requirements become resolved root edges |
| Transitive graph | Queued `.mod` metadata produces selected nodes and edges |
| Queue retries | Transient proxy failures use the existing retry lifecycle |
| Partial failure | Missing required metadata prevents report generation |
| DLQ behavior | Exhausted metadata work makes the audit incomplete |
| Durable shutdown | PostgreSQL jobs drain before selection and rendering finish |
| Report generation | Existing Markdown sections contain the Go module graph |
| License metadata | Go nodes explicitly carry `UNKNOWN` |
| npm/Python regression | Existing ecosystem suites remain unchanged |

## Validation Commands

```bash
go test -race ./internal/depfile ./internal/gomod ./cmd/auditor
go test -race ./...
go test -race -tags=integration ./...
go vet ./...
```

One optional manual smoke test may audit a small public Go module after all
deterministic tests pass.

## Exit Criteria

- [ ] Existing npm and Python behavior remains unchanged.
- [ ] Local and public-GitHub `go.mod` input is accepted as documented.
- [ ] Root module path, Go version, and requirements parse deterministically.
- [ ] Root replacement and exclusion behavior fails explicitly.
- [ ] Public dependency `.mod` files are fetched through the Go proxy protocol.
- [ ] Module path and version escaping follows `golang.org/x/mod/module`.
- [ ] Canonical releases, pseudo-versions, and `+incompatible` versions work.
- [ ] MVS selects one final version per module path.
- [ ] Module graph pruning follows the relevant Go directives.
- [ ] Unselected versions do not appear in final package or edge stores.
- [ ] Final edges target selected exact module coordinates.
- [ ] Fetch and worker completion order cannot change report bytes.
- [ ] Go module license metadata is explicitly reported as `UNKNOWN`.
- [ ] Incomplete, failed, or interrupted selection does not generate a report.
- [ ] No Go command, repository code, VCS client, or module archive is executed.
- [ ] Required deterministic, race-enabled, and PostgreSQL integration tests
      pass.

## Explicit Non-Goals for Completion

Completion of this feature does not mean the auditor reproduces every
`go list` build target, supports workspaces or replacements, verifies `go.sum`,
discovers licenses, scans source imports, or replaces the Go command. It means
the auditor can safely read one documented `go.mod`, retrieve public module
requirements through the Go proxy, apply deterministic MVS and graph-pruning
semantics, and render the resulting selected module dependency graph through
the existing audit pipeline.
