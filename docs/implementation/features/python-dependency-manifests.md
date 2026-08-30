# Feature Implementation — Python Dependency Manifests

## Status

Implemented. The required deterministic, race-enabled, and PostgreSQL
integration suites pass. The optional public-repository smoke test is not part
of required completion.

## Goal

Allow the auditor to use one Python dependency manifest as its audit seed while
preserving the existing durable traversal, policy evaluation, dependency graph,
and Markdown report lifecycle.

Supported examples:

```bash
go run ./cmd/auditor \
  --ecosystem python \
  --output audit-report.md \
  ./pyproject.toml
```

```bash
go run ./cmd/auditor \
  --ecosystem python \
  --output audit-report.md \
  ./requirements.txt
```

```bash
go run ./cmd/auditor \
  --ecosystem python \
  --manifest pyproject.toml \
  --output audit-report.md \
  https://github.com/owner/repository
```

`pip` is the Python package installer, not a manifest format. This feature reads
supported manifest syntax and retrieves metadata from public PyPI. It does not
execute `pip install`.

## Scope

### In scope

- Preserve the existing npm `package.json` behavior.
- Accept one local `pyproject.toml` or `requirements.txt` file.
- Accept one of those files from a public GitHub repository through the existing
  GitHub manifest source.
- Parse standard `[project].name` and `[project].dependencies` from
  `pyproject.toml`.
- Parse a safe, documented subset of `requirements.txt`.
- Parse package names and PEP 440 version constraints.
- Evaluate supported PEP 508 environment markers against an explicit,
  deterministic Python target.
- Resolve package versions and transitive metadata through public PyPI.
- Map resolved PyPI packages into the existing package and edge stores.
- Reuse the existing queue, worker, retry, DLQ, policy, shutdown, and Markdown
  report behavior.
- Add deterministic parser, resolver, handler, and CLI tests without contacting
  public GitHub or PyPI.

### Out of scope

- Python lockfiles, including `pylock.toml`, `poetry.lock`, `Pipfile.lock`, and
  generated pinned-environment formats.
- npm lockfiles or changes to npm resolution.
- Poetry-specific `[tool.poetry]` dependency tables.
- Pipenv `Pipfile` support.
- Conda environment files.
- Setuptools `setup.py` or `setup.cfg` dependency extraction.
- Dynamic `pyproject.toml` dependency metadata.
- `[project.optional-dependencies]` and standardized `[dependency-groups]` in
  the first implementation.
- Requirement extras such as `requests[socks]` in the first implementation.
- Recursive `-r` requirement files and `-c` constraint files.
- Editable installs, VCS dependencies, local paths, wheel paths, and direct
  archive URLs.
- Private PyPI indexes, mirrors, credentials, and arbitrary index URLs.
- Running Python, pip, build backends, package installers, or repository code.
- Full pip backtracking or reproduction of a complete installed environment.
- Automatic monorepo or manifest discovery.
- Auditing more than one ecosystem or manifest in a single command.
- Database migrations or changes to graph layout and Markdown rendering.

## Standards

The implementation must follow the current Python packaging specifications:

- `pyproject.toml` project metadata:
  <https://packaging.python.org/specifications/declaring-project-metadata/>
- Dependency specifiers and environment markers:
  <https://packaging.python.org/en/latest/specifications/dependency-specifiers/>
- Version specifiers:
  <https://packaging.python.org/en/latest/specifications/version-specifiers/>
- Package-name normalization:
  <https://packaging.python.org/en/latest/specifications/name-normalization/>
- pip requirements-file syntax:
  <https://pip.pypa.io/en/stable/reference/requirements-file-format/>
- PyPI JSON API:
  <https://docs.pypi.org/api/json/>

## CLI Contract

Add an ecosystem selector:

```text
--ecosystem <npm|python>
```

Rules:

- `--ecosystem` defaults to `npm` to preserve every existing command.
- Python local input must be `pyproject.toml` or `requirements.txt`.
- Python GitHub input requires an explicit `--manifest` selecting one of those
  supported filenames.
- `--ref` retains its existing GitHub-only meaning.
- `--manifest` remains valid only for GitHub input.
- Exactly one positional local path or GitHub repository URL remains required.
- The auditor does not search a repository for Python manifests.
- An unsupported or ambiguous manifest filename returns a clear error.

Add deterministic Python target options:

| Option | Default | Behavior |
|---|---|---|
| `--python-version` | `3.12` | Value used for `python_version` and `python_full_version` markers |
| `--python-platform` | `linux` | Value used to derive supported operating-system markers |

These options are valid only with `--ecosystem python`. The chosen target must
appear in the terminal summary and Markdown report metadata; it must not change
the dependency graph layout.

Examples:

```bash
go run ./cmd/auditor \
  --ecosystem python \
  --python-version 3.12 \
  --python-platform linux \
  --manifest requirements.txt \
  --output audit-report.md \
  https://github.com/owner/repository
```

## Manifest Selection

Manifest format is selected after CLI validation:

| Ecosystem | Manifest | Parser |
|---|---|---|
| `npm` | `package.json` | Existing npm JSON parser |
| `python` | `pyproject.toml` | Python TOML project parser |
| `python` | `requirements.txt` | Safe pip-requirements subset parser |

Selection must use the logical manifest basename, not content guessing. A file
with an unsupported name must not be silently interpreted as another format.

## Shared Manifest Result

Both Python parsers return the existing manifest boundary:

```go
type Manifest struct {
    Name         string
    Dependencies []Dependency
}
```

For Python, `Dependency.VersionRange` stores a normalized PEP 440 constraint.
The existing npm parser continues to store an npm semver range. Interpretation
of that value belongs to the registry client selected for the audit.

Dependency output must be sorted by normalized package name and then constraint
so that seed-job order remains deterministic.

## `pyproject.toml` Contract

Read only standardized project metadata:

```toml
[project]
name = "example-service"
dependencies = [
  "requests>=2.31,<3",
  "flask~=3.0",
  "colorama; sys_platform == 'win32'",
]
```

Rules:

- `[project].name` becomes the audit root.
- `[project].dependencies` must be an array of PEP 508 strings.
- Each active requirement becomes one direct dependency.
- Environment markers are evaluated against the configured Python target.
- Requirements whose markers evaluate false are omitted.
- Missing `[project].dependencies` preserves the existing no-work behavior.
- Missing or empty `[project].name` is invalid for `pyproject.toml` input.
- If `project.dynamic` contains `dependencies`, return an unsupported-dynamic-
  metadata error; do not invoke a build backend.
- Optional dependencies, dependency groups, tool-specific tables, direct URLs,
  and extras return a clear unsupported-feature error when encountered in the
  selected dependency set.

Use a TOML parser library. Do not parse TOML with regular expressions.

## `requirements.txt` Contract

The first implementation accepts:

```text
# Runtime dependencies
requests>=2.31,<3
flask~=3.0
colorama==0.4.6 ; sys_platform == "win32"
```

Supported syntax:

- UTF-8 text;
- blank lines;
- full-line and trailing comments;
- escaped line continuations;
- normalized package names;
- PEP 440 constraints;
- supported PEP 508 environment markers.

Rejected syntax:

- `-r` and `--requirement` includes;
- `-c` and `--constraint` includes;
- `-e` and `--editable` entries;
- local or relative paths;
- VCS references;
- direct URL requirements;
- package extras;
- index, trusted-host, binary-selection, hash-mode, and other pip options;
- environment-variable substitution.

An unsupported line returns its line number and a sanitized reason. The parser
must not attempt to behave like the pip CLI beyond this declared subset.

Because `requirements.txt` has no project-name field, its audit root is:

- the GitHub repository name for GitHub input; or
- the parent directory name for local input.

## Python Target and Markers

Marker evaluation must be deterministic and must not inspect a locally
installed Python interpreter. Construct a fixed marker environment from the CLI
target, including at least:

- `python_version`;
- `python_full_version`;
- `sys_platform`;
- `os_name`;
- `platform_system`;
- `implementation_name` set to `cpython`.

Reject marker expressions that require a target property the implementation
does not define. Never evaluate markers with a general-purpose expression
engine or execute their text as code.

The initial platform values are limited to documented mappings for `linux`,
`windows`, and `darwin`. Other values return a CLI validation error.

## PyPI Registry Contract

Add a `RegistryClient` implementation for public PyPI. It must satisfy the
existing concurrent registry boundary and return `PackageMetadata` with an exact
version, license, and active direct dependencies.

Resolution flow for each requirement:

1. Normalize the package name according to the Python name-normalization spec.
2. Parse the incoming PEP 440 constraint.
3. Fetch available releases from:

   ```text
   GET https://pypi.org/pypi/{normalized-name}/json
   ```

4. Ignore invalid and withdrawn releases.
5. Select the highest release satisfying the constraint and prerelease rules.
6. Fetch exact release metadata from:

   ```text
   GET https://pypi.org/pypi/{normalized-name}/{version}/json
   ```

7. Parse `info.requires_dist` as PEP 508 requirements.
8. Evaluate each dependency marker against the configured target.
9. Return active dependencies as normalized name-to-constraint entries for the
   existing lazy child-job traversal.

Production defaults:

- base URL: `https://pypi.org`;
- request timeout: 15 seconds;
- maximum metadata response: 5 MiB;
- no automatic retries inside the registry client;
- injectable base URL and HTTP client for deterministic tests.

This is a metadata audit resolver. Like the existing npm traversal, it resolves
each dependency edge lazily. It does not claim to reproduce pip's global
backtracking result or produce an installable lock.

## Version Handling

Python versions must not use the existing npm semver parser. Add a dedicated
PEP 440 boundary:

```go
type PythonVersionResolver interface {
    Resolve(constraint string, available []string) (string, error)
}
```

It must support the version operators accepted in the scoped input:

- exact match: `==` and `===`;
- exclusion: `!=`;
- ordered comparison: `<`, `<=`, `>`, and `>=`;
- compatible release: `~=`;
- comma-separated intersections;
- wildcard equality where permitted by PEP 440;
- prerelease filtering according to PEP 440.

Do not translate PEP 440 constraints into npm semver syntax.

## License Metadata

For an exact PyPI release, derive one license string in this order:

1. standardized license expression when present;
2. non-empty `info.license` value;
3. recognized `License ::` classifiers;
4. `UNKNOWN` when no usable metadata exists.

The derived string is passed to the existing policy checker. Any future changes
to license-policy semantics require a separate feature document.

## Existing Pipeline Integration

One audit run selects one parser and one registry client:

```text
CLI ecosystem + manifest
        ↓
local/GitHub ManifestSource
        ↓
npm parser or Python parser
        ↓
NpmClient or PyPIClient
        ↓
existing audit jobs and workers
        ↓
existing package/edge stores
        ↓
existing terminal and Markdown reports
```

The current `RegistryClient` interface already permits selecting one registry
implementation per audit. The first Python implementation must not add an
ecosystem switch to every job, because mixed-ecosystem audits are out of scope.

No job-table or DLQ schema migration is required. Existing npm payloads and npm
graph coordinates must remain unchanged.

## Execution Flow

1. Parse and validate CLI ecosystem and target options.
2. Classify the positional input as local or GitHub.
3. Read or fetch exactly one selected manifest.
4. Select the parser from ecosystem and manifest basename.
5. Parse the project root and deterministic direct dependencies.
6. Reject invalid or unsupported input before opening PostgreSQL.
7. Select `NpmClient` or `PyPIClient` for the entire audit.
8. Run the existing durable audit lifecycle.
9. Generate the existing terminal and optional Markdown reports.

## Error Handling

Return clear errors for:

- unsupported ecosystem or manifest combinations;
- malformed TOML or requirements syntax;
- missing required `pyproject.toml` project metadata;
- dynamic dependency metadata;
- unsupported requirements directives or sources;
- malformed PEP 440 constraints or PEP 508 markers;
- unsupported marker target properties;
- missing PyPI projects or releases;
- constraints with no matching release;
- PyPI rate limits and non-success HTTP status codes;
- DNS, connection, TLS, timeout, decode, and response-size failures;
- malformed `Requires-Dist` metadata.

Errors may identify the public package, version, manifest, and line number. They
must not include credentials, authorization headers, or response bodies.

## Security Constraints

- Fetch manifests only through the existing local-file and validated public
  GitHub source boundaries.
- Fetch package metadata only from the fixed public PyPI base URL in production.
- Never accept a package index URL from manifest contents.
- Never execute Python, pip, build backends, package hooks, or repository code.
- Never download or unpack wheels or source distributions.
- Never resolve local paths, VCS sources, or direct archive URLs.
- Apply fixed request timeouts and response-size limits.
- Parse markers with a dedicated standards-aware parser.
- Do not write downloaded manifests or registry responses to disk.

## Component Changes

```text
cmd/auditor/
├── main.go                         ← ecosystem/target selection and registry choice
└── main_test.go                    ← CLI and orchestration tests
internal/depfile/
├── python.go                       ← Python manifest selection boundary
├── pyproject.go                    ← standardized TOML parser
├── requirements.go                ← safe requirements subset parser
└── *_test.go                       ← deterministic parser fixtures
internal/pypi/
├── client.go                       ← public PyPI metadata client
├── requirement.go                  ← PEP 508 subset and marker handling
├── version.go                      ← PEP 440 parsing and resolution
└── *_test.go                       ← httptest and resolver coverage
```

Expected existing-file changes are limited to registry selection and any
minimal shared interfaces required to pass Python constraints through the
existing single-ecosystem traversal. Do not change queue capacity, worker
scheduling, persistence schema, npm behavior, graph mapping, or Markdown graph
layout.

## Testing Strategy

### CLI and source tests

| Test | Verification |
|---|---|
| Existing npm command | Defaults to npm with no behavior change |
| Local pyproject | Selects the standardized TOML parser |
| Local requirements | Selects the requirements parser |
| GitHub pyproject | Existing GitHub source returns TOML bytes |
| GitHub requirements | Existing GitHub source returns text bytes |
| Unsupported manifest | Fails before PostgreSQL opens |
| Python-only options | Rejected for npm input |
| Target validation | Unsupported version/platform values are rejected |

### Parser tests

| Test | Verification |
|---|---|
| Project metadata | Name and dependencies are parsed |
| Deterministic order | Dependencies are normalized and sorted |
| Missing dependencies | Produces the existing no-work result |
| Dynamic dependencies | Rejected without running a build backend |
| Requirements comments | Comments and blank lines are ignored |
| Line continuation | Continued requirements are parsed once |
| Environment marker | Included or excluded for the configured target |
| Unsupported directive | Returns a line-specific error |
| URL/VCS/local source | Rejected without fetching or executing it |
| Extras | Rejected clearly in the first implementation |

### Version and PyPI tests

| Test | Verification |
|---|---|
| PEP 440 operators | Exact, range, compatible, exclusion, and wildcard cases |
| Prereleases | Stable/prerelease selection follows PEP 440 |
| Name normalization | Case, hyphen, underscore, and period variants normalize |
| Highest match | Resolver selects the highest satisfying version |
| No match | Clear constraint error is returned |
| Exact metadata | Exact-release endpoint is requested |
| Transitive dependencies | Active `Requires-Dist` entries become child metadata |
| Marker filtering | Inactive target-specific dependencies are omitted |
| License precedence | Expression, legacy value, classifier, then `UNKNOWN` |
| HTTP failures | `404`, `429`, and other failures are clear |
| Timeout and size | Fixed safety limits are enforced |
| No public network | All HTTP behavior uses `httptest.Server` |

### Integration tests

| Test | Verification |
|---|---|
| Python seed mapping | Root-to-direct dependency edges use resolved versions |
| Python transitive graph | PyPI metadata produces deterministic nodes and edges |
| Policy evaluation | Python license metadata reaches the existing checker |
| Report generation | Existing Markdown output contains the Python graph |
| Durable lifecycle | PostgreSQL jobs drain successfully without schema changes |
| npm regression | Existing npm integration behavior remains unchanged |

## Validation Commands

```bash
go test -race ./internal/depfile ./internal/pypi ./cmd/auditor
go test -race ./...
go test -race -tags=integration ./...
```

One optional manual smoke test may audit a small public Python repository after
all deterministic tests pass.

## Exit Criteria

- [x] Existing npm local and GitHub inputs behave as before.
- [x] A local `pyproject.toml` produces Python seed dependencies.
- [x] A local `requirements.txt` produces Python seed dependencies.
- [x] Public GitHub input can select either supported Python manifest.
- [x] PEP 440 constraints resolve to exact public PyPI versions.
- [x] Supported environment markers are deterministic for the configured target.
- [x] Active transitive `Requires-Dist` metadata produces graph edges.
- [x] Python package names are normalized consistently.
- [x] Python license metadata reaches the existing policy checker.
- [x] Unsupported executable, local, VCS, URL, and dynamic inputs fail safely.
- [x] No Python or repository code is executed.
- [x] npm behavior and output remain unchanged.
- [x] No database migration is added.
- [x] Required deterministic and race-enabled tests pass.
- [x] PostgreSQL integration tests pass.

## Explicit Non-Goals for Completion

Completion of this feature does not mean the auditor is a pip replacement,
lockfile generator, vulnerability scanner, Python installer, or universal
Python project detector. It means the auditor can safely read the documented
subset of two Python manifest formats and traverse public PyPI metadata through
the existing audit pipeline.
