# GitHub Python File Dependency Graph — Implementation Plan

## Status

Implemented.

## Objective

Allow file-analysis mode to accept one public GitHub repository URL, download
the repository temporarily, analyze its Python source-file imports, and write a
deterministic JSON file dependency graph.

Target command:

```bash
go run ./cmd/auditor \
  --analysis files \
  --output pc-diagnostic-file-graph.json \
  https://github.com/thomxsnguyen/pc_diagnostic
```

Optional branch, tag, or commit:

```bash
go run ./cmd/auditor \
  --analysis files \
  --ref main \
  --output pc-diagnostic-file-graph.json \
  https://github.com/thomxsnguyen/pc_diagnostic
```

The user does not clone the repository manually. The CLI owns download,
temporary extraction, analysis, report writing, and cleanup.

## Fixed Scope

### Included

- One public `https://github.com/{owner}/{repository}` URL per run.
- The repository default branch or one explicit `--ref`.
- One bounded repository archive download.
- Safe extraction into an application-created temporary directory.
- Python files ending in `.py`.
- Python `import` and `from ... import ...` statements.
- Absolute imports that resolve to files within the downloaded repository.
- Explicit relative imports that resolve within the importing package.
- Common project-root and `src/` Python package layouts.
- One existing `analyze_file` queue job per discovered Python file.
- Deterministic JSON nodes, edges, and diagnostics.
- Cleanup of the temporary repository after report generation or failure.
- Unit tests using local fixtures and `httptest`; tests do not call GitHub.

### Excluded

- UI changes or combined package/file graph presentation.
- Package dependency, license, or vulnerability auditing changes.
- Scanning `.venv`, `venv`, `site-packages`, or other installed dependencies.
- Private repositories, GitHub Apps, OAuth, or new authentication flows.
- GitHub Enterprise or non-GitHub hosts.
- Repository cloning or invoking the `git` executable.
- Git submodules and Git LFS object retrieval.
- Multiple repositories or monorepo package selection.
- Dynamic imports through `importlib`, `__import__`, or computed module names.
- Python namespace packages without `__init__.py`.
- Mapping external Python import names to distribution package names.
- JavaScript/TypeScript behavior changes.
- New database tables, migrations, or durable file-graph result storage.
- Changes to queue or worker interfaces.
- Watch mode, caching, incremental analysis, call graphs, or symbol graphs.

## Architecture

```text
GitHub repository URL
        |
        v
validate owner/repository and optional ref
        |
        v
download one ZIP archive with size limit
        |
        v
extract safely into os.MkdirTemp directory
        |
        v
discover .py files and build immutable file index
        |
        v
one analyze_file job per Python file
        |
        v
existing in-memory queue and bounded worker pool
        |
        v
Python import extraction and local module resolution
        |
        v
deterministic file-graph JSON
        |
        v
remove temporary directory
```

GitHub acquisition and Python graph analysis remain separate components. The
GitHub client returns an extracted repository root; it does not parse Python or
write graph results. The file graph handler does not know that its files came
from GitHub.

## Queue Decision

Use the existing queue in its in-memory mode:

```go
q := queue.New(bufferSize)
```

Do not require `DATABASE_URL` for file analysis.

Durable jobs are inappropriate for this temporary-source workflow because a
restarted process cannot safely resume a job whose temporary directory has
already been removed. The in-memory queue still provides bounded concurrency
and keeps one job per file without changing queue or worker architecture.

Package analysis continues to use the PostgreSQL-backed queue unchanged.

## CLI Contract

File mode accepts either an existing local directory or a public GitHub URL:

```text
auditor --analysis files --output <path> [--ref <value>] <directory-or-github-url>
```

Rules for GitHub file analysis:

- `--output` is required and non-empty;
- the URL must pass the existing GitHub repository URL validation;
- `--ref` is optional and valid only with GitHub input;
- `--manifest`, `--ecosystem`, and Python target options remain invalid in
  file mode;
- `.git`, `/tree/...`, and `/blob/...` URL forms are not inferred beyond the
  repository URL normalization already supported;
- existing package-analysis CLI behavior remains unchanged.

## GitHub Archive Acquisition

Add a repository archive operation to `internal/github` using GitHub's ZIP
archive endpoint:

```http
GET /repos/{owner}/{repository}/zipball/{ref}
Accept: application/vnd.github+json
```

When no ref is supplied, request the default-branch archive endpoint without a
ref. The HTTP client must follow GitHub's redirect to the archive download.

Reference:

- <https://docs.github.com/en/rest/repos/contents#download-a-repository-archive-zip>

The downloader must:

- use the existing bounded HTTP timeout;
- identify itself with the existing user agent;
- return clear not-found, rate-limit, HTTP-status, and read errors;
- cap compressed response bytes before buffering the archive;
- never log request authorization headers or response bodies;
- accept an injectable base URL and HTTP client for tests.

## Safe ZIP Extraction

Extract into a directory created by `os.MkdirTemp`. Never extract directly into
the workspace or current working directory.

For every archive entry:

1. Normalize the slash-separated entry name.
2. Reject absolute paths.
3. Reject `.` and `..` traversal outside the temporary directory.
4. Reject symbolic links and unsupported special files.
5. Enforce a per-file size limit.
6. Enforce a total extracted-size limit.
7. Enforce a maximum archive-entry count.
8. Create only regular files and directories under the temporary root.

GitHub archives contain one generated top-level directory. Extraction returns
that directory as the repository root and rejects archives containing multiple
unrelated top-level roots.

The caller owns cleanup:

```go
temporaryDirectory, err := os.MkdirTemp("", "auditor-github-*")
if err != nil { /* return */ }
defer os.RemoveAll(temporaryDirectory)
```

Cleanup executes after either successful report writing or an error.

## Python File Discovery

Extend `internal/filegraph.Discover` to include regular `.py` files while
preserving current JavaScript and TypeScript discovery.

Add these excluded directory names:

```text
.venv
venv
__pycache__
.pytest_cache
.mypy_cache
.ruff_cache
```

Existing exclusions such as `.git`, `node_modules`, `dist`, `build`, and
`coverage` remain unchanged.

Every discovered Python path is normalized relative to the extracted
repository root, sorted, inserted as a graph node, and submitted as one
`analyze_file` job.

## Python Import Model

Add a Python-specific import representation without changing the existing
JavaScript/TypeScript extractor API:

```go
type PythonImport struct {
    Module string
    Level  int
    Names  []string
}
```

Examples:

| Source | Parsed representation |
|---|---|
| `import pc_diagnostic.cache` | module `pc_diagnostic.cache`, level `0` |
| `import pc_diagnostic.cache as cache` | module `pc_diagnostic.cache`, level `0` |
| `import first, second` | two independent imports |
| `from pc_diagnostic.models import Snapshot` | module `pc_diagnostic.models`, level `0` |
| `from .bridge import TelemetryBridge` | module `bridge`, level `1` |
| `from ..models import Snapshot` | module `models`, level `2` |
| `from . import helpers` | empty module, level `1`, name `helpers` |

The extractor recognizes imports at module or nested block indentation. It
must ignore import-like text in comments and string literals.

Parenthesized imported-name lists do not change the module dependency:

```python
from pc_diagnostic.models import (
    MetricReading,
    Snapshot,
)
```

This produces one dependency on `pc_diagnostic.models`.

## Python Parser Boundary

Add a focused Python import tokenizer/parser in `internal/filegraph` rather
than using unrestricted regular expressions over complete source files.

The parser only needs to understand the import grammar listed in this
document. It does not build a Python AST, resolve symbols, or execute source.

Malformed supported import syntax becomes a file diagnostic and completes the
job successfully. Deterministic source syntax problems must not consume the
queue's retry allowance.

## Python Module Resolution

Resolution uses only the immutable discovered-file index. It never imports or
executes repository code.

### Absolute imports

For:

```python
from pc_diagnostic.models import Snapshot
```

try these candidates in order:

```text
pc_diagnostic/models.py
pc_diagnostic/models/__init__.py
src/pc_diagnostic/models.py
src/pc_diagnostic/models/__init__.py
```

This supports conventional project-root and `src/` layouts, including the
layout used by `pc_diagnostic`.

### Explicit relative imports

Resolve the leading-dot level from the importing file's package directory:

```text
src/pc_diagnostic/gui/app.py
from .bridge import TelemetryBridge

→ src/pc_diagnostic/gui/bridge.py
```

For `from . import helpers`, try the named submodule and subpackage beneath the
current package. If the name is not a module but the current package has an
`__init__.py`, record the dependency on that `__init__.py` rather than
inventing a missing file.

Reject relative imports that ascend above the repository root.

### External imports

Imports such as these are not file edges:

```python
import os
import psutil
from rich.console import Console
```

Determine local top-level module names from indexed root packages, root module
files, and `src/` packages. Ignore imports whose top-level name is not local.

If an import begins with a known local top-level module but no candidate file
exists, record an unresolved-local-import diagnostic. Do not create package
nodes or connect to the package dependency graph in this feature.

## Handler Integration

Keep the existing `analyze_file` job type and payload. In
`filegraph.Handler.Handle`, select extraction and resolution by file extension:

```text
.js/.jsx/.ts/.tsx → existing JavaScript/TypeScript path
.py                → Python import path
```

The Python path performs:

1. Read the source through the existing bounded file reader.
2. Extract Python imports.
3. Resolve local modules through the shared index.
4. Add each resolved `file -> file` edge to the existing graph store.
5. Ignore confirmed external imports.
6. Record diagnostics for unresolved local imports or parsing failures.
7. Return no child jobs.

The graph node, edge, diagnostic, store, report, and JSON schemas remain
unchanged.

## Output Contract

The existing deterministic file graph JSON remains the only output:

```json
{
  "root": "pc_diagnostic",
  "nodes": [
    { "path": "src/pc_diagnostic/main.py" },
    { "path": "src/pc_diagnostic/models.py" }
  ],
  "edges": [
    {
      "from": "src/pc_diagnostic/main.py",
      "to": "src/pc_diagnostic/models.py"
    }
  ],
  "diagnostics": []
}
```

The GitHub archive's generated directory name must not become the report root.
Use the normalized GitHub repository name instead.

## Component Changes

```text
cmd/auditor/
├── main.go                         <- accept GitHub URL in files mode,
│                                      acquire/cleanup archive, use in-memory queue
└── main_test.go                    <- CLI and orchestration tests

internal/github/
├── archive.go                      <- bounded ZIP download and safe extraction
└── archive_test.go                 <- httptest and malicious archive cases

internal/filegraph/
├── discovery.go                    <- include .py and Python cache exclusions
├── discovery_test.go
├── python_extractor.go             <- parse supported Python imports
├── python_extractor_test.go
├── python_resolver.go              <- root/src and relative resolution
├── python_resolver_test.go
├── handler.go                      <- dispatch extraction by extension
└── handler_test.go
```

Do not modify:

```text
internal/job/
internal/queue/
internal/worker/
internal/dlq/
internal/store/
internal/auditor/
internal/depfile/
internal/pypi/
db/migrations/
web/
```

## Testing Plan

### GitHub archive tests

| Test | Verification |
|---|---|
| Default branch | Requests the repository ZIP endpoint without a ref |
| Explicit ref | Safely encodes and requests the selected ref |
| Public download | Follows the archive redirect and reads the ZIP |
| Not found | Returns a repository-specific error |
| Rate limit | Returns a clear rate-limit error |
| Compressed size | Rejects an oversized response |
| Path traversal | Rejects `../` and absolute archive entries |
| Symlink | Rejects symbolic-link entries |
| Extracted size | Rejects oversized extracted contents |
| Entry count | Rejects archives with too many entries |
| Multiple roots | Rejects an unexpected archive layout |
| Cleanup | Removes the temporary directory after success and failure |

### Python extraction tests

Cover:

- absolute `import`;
- aliased imports;
- comma-separated imports;
- absolute `from` imports;
- one-level and multi-level relative imports;
- `from . import name`;
- parenthesized imported names;
- imports inside conditional or function blocks;
- trailing comments;
- import-like text inside strings and comments;
- malformed supported import syntax.

### Python resolution tests

Cover:

- root module files;
- root packages;
- `src/` module files;
- `src/` packages;
- nested modules;
- relative sibling modules;
- relative parent-package modules;
- package `__init__.py`;
- repository-root escape rejection;
- external import exclusion;
- unresolved known-local import diagnostics.

### End-to-end fixture

Build a ZIP archive in memory with this shape:

```text
thomxsnguyen-pc_diagnostic-test/
├── pyproject.toml
├── src/pc_diagnostic/__init__.py
├── src/pc_diagnostic/main.py
├── src/pc_diagnostic/cache.py
├── src/pc_diagnostic/models.py
├── tests/test_cache.py
└── .venv/lib/python/site-packages/ignored.py
```

Use `httptest.Server` as the GitHub API and archive host. Run the real CLI
orchestration with the existing in-memory queue and assert:

- the public GitHub URL is accepted;
- no `DATABASE_URL` is required;
- every repository `.py` file outside exclusions becomes one node;
- absolute `src/` imports resolve;
- relative imports resolve;
- tests can point to application source files;
- external imports are absent;
- the virtual-environment file is absent;
- JSON output is deterministic;
- the temporary directory is deleted.

Tests must not call GitHub or depend on network access.

## Implementation Sequence

1. Extend discovery for `.py` and Python-specific excluded directories.
2. Add the Python import representation and focused extractor.
3. Add root-layout, `src/`-layout, and relative Python resolution.
4. Route `.py` jobs through the Python handler path.
5. Add bounded GitHub ZIP download and safe extraction.
6. Allow GitHub URLs and `--ref` in file-analysis CLI parsing.
7. Switch file-analysis orchestration to the existing in-memory queue.
8. Add focused unit and end-to-end tests.
9. Run formatting, race tests, the complete Go suite, and a changed-file scope
   audit.

## Acceptance Criteria

- A public GitHub Python repository URL can be analyzed without a manual clone.
- `pc_diagnostic`-style `src/` imports resolve to repository-relative `.py`
  paths.
- Each discovered Python file is processed by one existing queue job.
- File analysis does not require PostgreSQL or `DATABASE_URL`.
- Standard-library and third-party imports do not become file nodes or edges.
- Virtual environments, caches, build outputs, and installed dependency files
  are excluded.
- The downloaded repository exists only in a temporary directory and is always
  cleaned up.
- Unsafe or oversized archives fail before files can escape configured limits.
- Existing JavaScript/TypeScript file analysis remains unchanged.
- Existing package auditing remains unchanged.
- Output remains deterministic JSON using the current schema.
- No UI, combined graph, database schema, queue contract, or worker contract is
  changed.
