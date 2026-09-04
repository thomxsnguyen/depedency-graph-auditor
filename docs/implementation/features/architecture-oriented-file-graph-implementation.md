# Architecture-Oriented File Graph — Implementation Plan

## Status

Implemented according to the scoped plan below.

## Objective

Organize the existing file dependency graph around recognizable software
architecture instead of treating every directory as an equally important
module.

The Files canvas will present the same normalized file-to-file dependency data
at three progressive levels:

```text
Architecture view -> Domain view -> File view
```

The default view communicates the repository's overall dependency flow. Users
drill into one architectural group, then one domain, and finally inspect
individual files with the existing one-hop or two-hop focus.

## Hard Scope Boundary

This implementation includes only:

- frontend-only architectural classification derived from existing file paths;
- deterministic convention profiles for the currently supported
  JavaScript/TypeScript, Python, and Go file extensions;
- deterministic project, architectural-layer, and domain grouping;
- progressive architecture, domain, and file expansion in the existing Files
  canvas;
- aggregation of existing file edges at the currently visible level;
- a left-to-right layout ordered by architectural responsibility;
- separate supporting lanes for configuration, shared code, tests, and tools;
- a restrained neutral arrow palette for relationship readability;
- preservation of existing file search, diagnostics, category colors, file
  inspector, minimap, and dependency-hop focus; and
- focused unit, component, and browser coverage.

It does not include:

- analyzer, resolver, discovery, queue, worker, retry, or DLQ changes;
- HTTP endpoint or `FileGraphSnapshot` schema changes;
- reading manifests or source files in the browser;
- backend architectural classification;
- user-defined mappings or repository configuration;
- machine-learning or AI classification;
- package/file graph combination;
- function, symbol, class, call, runtime, or data-flow analysis;
- architecture-policy enforcement or violation reporting;
- new analyzer or language support beyond the existing JavaScript/TypeScript,
  Python, and Go file graph;
- framework-specific conventions such as React, Next.js, Django, Flask, or
  Gin;
- persistence, migrations, caching, pagination, or virtualization;
- changes to package dependency mode;
- redesign of the surrounding application; or
- deployment work.

## Existing Behavior to Preserve

- The backend remains the source of normalized file nodes, file edges, and
  diagnostics.
- JavaScript/TypeScript, Python, and Go repositories continue to use their
  current analyzers.
- Local and GitHub repository input remain unchanged.
- The file-analysis queue and worker count remain unchanged.
- The current module graph implementation remains the foundation for visible
  graph aggregation; this work replaces only its path-grouping policy and
  expansion hierarchy.
- File dependency direction remains `from imports to`.
- Aggregate edge counts must still equal the original file edges represented by
  each visible relationship.
- Selecting a file continues to default to one dependency hop and allows two
  hops or all currently expanded files.
- Loading a new repository clears file-graph expansion, selection, position,
  viewport, and hop state.
- Package dependency mode remains unchanged and remains the initial app mode.

## Resulting User Flow

```text
Open Files
    |
    v
Architecture groups and overall relationships
    |
    | expand one group
    v
Domains/features inside that group
    |
    | expand one domain
    v
Files inside that domain
    |
    | select one file
    v
1 hop -> 2 hops -> All
```

Example:

```text
Presentation -> Transport -> Services -> Domain -> Persistence
                                  |
                                  v
                      expand Services
                       |- authentication
                       |- diagnostics
                       `- reporting
                                  |
                                  v
                      expand diagnostics
                       |- cpu_service.py
                       |- memory_service.py
                       `- system_info.py
```

Configuration, tests, shared code, and tooling are supporting groups rather
than additional stages in the main left-to-right application flow.

## Architecture

```text
existing FileGraphSnapshot
          |
          v
path-only architecture classifier
  - project
  - layer
  - domain
          |
          v
hierarchical visible-graph transformation
  - architecture nodes
  - domain nodes for expanded layers
  - file nodes for expanded domains
          |
          v
relationship aggregation and neutral edge classification
          |
          v
ordered React Flow layout
```

The transformation must be pure and deterministic. It must not mutate the
source snapshot or invent dependencies.

## Files to Add

```text
web/src/graph/fileArchitecture.ts
web/src/components/FileGraphCanvas/ArchitectureNode.tsx
web/src/components/FileGraphCanvas/DomainNode.tsx
web/tests/fileArchitecture.test.ts
```

## Files to Change

```text
web/src/graph/hierarchicalFileGraph.ts
web/src/graph/mapFileGraph.ts
web/src/graph/layoutGraph.ts
web/src/app/AuditWorkspace.tsx
web/src/components/FileGraphCanvas/FileGraphCanvas.tsx
web/src/styles/globals.css
web/tests/hierarchicalFileGraph.test.ts
web/tests/mapFileGraph.test.ts
web/tests/components.test.tsx
web/tests/browser/audit-studio.spec.ts
docs/implementation/features/hierarchical-file-graph-implementation.md
```

Do not modify Go source, API handlers, data sources, JSON report types, package
graph components, deployment files, or database files.

## Step 1 — Define the Architectural Vocabulary

Use one small, fixed vocabulary:

```ts
type ArchitectureLayer =
  | "entrypoint"
  | "presentation"
  | "transport"
  | "application"
  | "domain"
  | "persistence"
  | "infrastructure"
  | "shared"
  | "configuration"
  | "test"
  | "tooling"
  | "other"

interface FileArchitecture {
  filePath: string
  project: string
  layer: ArchitectureLayer
  domain: string
  classification: "universal" | "language-convention" | "generic" | "fallback"
  confidence: "inferred" | "fallback"
}
```

This metadata is frontend-only and must not be added to the backend report.

Use these display labels:

| Layer | Display label |
|---|---|
| `entrypoint` | Entrypoints |
| `presentation` | Presentation |
| `transport` | Transport / API |
| `application` | Services |
| `domain` | Domain |
| `persistence` | Persistence |
| `infrastructure` | Infrastructure |
| `shared` | Shared |
| `configuration` | Configuration |
| `test` | Tests |
| `tooling` | Tooling |
| `other` | Other |

Do not add repository-specific layer names during this implementation.

## Step 2 — Classify Project Boundaries From Paths

The current snapshot contains supported source-file paths, not manifest nodes.
Therefore, project boundaries must be inferred only from path segments.

Use these deterministic rules in order:

1. `apps/<name>/...`, `packages/<name>/...`, and `services/<name>/...` use
   `<first segment>/<name>` as the project.
2. A leading `frontend`, `backend`, `client`, `server`, `web`, `api`, or `cli`
   segment is the project.
3. Any other file below a directory uses its first directory segment as the
   project boundary.
4. Top-level files belong to the synthetic project `repository`.

Examples:

```text
apps/admin/src/App.tsx             -> apps/admin
packages/auth/src/token.ts         -> packages/auth
frontend/components/Button.tsx     -> frontend
backend/services/users.py          -> backend
internal/config/load.go            -> internal
engine/runtime/tasks/runner.py     -> engine
main.go                            -> repository
```

Do not inspect `package.json`, `pyproject.toml`, `go.mod`, workspace files, or
package declarations. Manifest-aware project detection requires backend data
that is outside this scope.

## Step 3 — Select a Convention Profile Per File

Select the optional convention profile from the normalized file extension:

```text
.js, .jsx, .ts, .tsx -> javascript-typescript
.py                   -> python
.go                   -> go
anything else         -> no language profile
```

Selection is per file, not per repository. A polyglot repository may therefore
use all three profiles in the same graph. Profile selection must not call an
analyzer or inspect source contents.

Profiles only contribute candidate architectural labels. They never remove,
rename, or relocate the original directory path.

## Step 4 — Classify Architectural Layers

Apply classifiers in this order:

```text
universal conventions
        -> selected language convention profile
        -> generic architecture conventions
        -> directory fallback
```

Within a classifier, resolve competing matches with this fixed role
precedence:

```text
test
  -> configuration
  -> tooling
  -> persistence
  -> transport
  -> presentation
  -> domain
  -> application
  -> infrastructure
  -> shared
  -> entrypoint
  -> other
```

### Universal conventions

Apply these rules before selecting a language-specific role:

| Layer | Path segments or filenames |
|---|---|
| Tests | `test`, `tests`, `__tests__`, `spec`, `specs`, `*.test.*`, `*.spec.*`, `test_*.py`, `*_test.go` |
| Configuration | `config`, `configs`, `configuration`, `.github`, `settings.*`, `*.config.*` |
| Tooling | `script`, `scripts`, `tool`, `tools`, `bin` |

Generated and vendor files retain the existing generated category and remain
visually de-emphasized. They do not introduce an additional architectural
layer in this implementation.

### JavaScript and TypeScript profile

Use only for `.js`, `.jsx`, `.ts`, and `.tsx` files:

| Convention | Layer |
|---|---|
| `pages`, `screens`, `views`, `components`, `ui`, `hooks` | Presentation |
| `routes`, `controllers`, `middleware` | Transport / API |
| `services`, `use-cases`, `usecases` | Services |
| `store`, `state`, `reducers` | Services |
| `models`, `entities`, `domain` | Domain |
| `api`, `clients` | Infrastructure |
| `utils`, `lib`, `shared` | Shared |
| `main.*`, `index.*`, `app.*`, `server.*` | Entrypoints |

State-management directories map to the existing Services layer; do not add a
new State layer. `api` maps to Infrastructure when it is inside a recognized
frontend/client project and to Transport through the generic rules otherwise.

### Python profile

Use only for `.py` files:

| Convention | Layer |
|---|---|
| `views`, `templates`, `gui`, `widgets` | Presentation |
| `api`, `routes`, `endpoints` | Transport / API |
| `services`, `use_cases`, `usecases` | Services |
| `models`, `entities`, `domain`, `schemas`, `serializers` | Domain |
| `repositories`, `db`, `database`, `migrations` | Persistence |
| `providers`, `adapters`, `integrations` | Infrastructure |
| `utils`, `helpers`, `common` | Shared |
| `main.py`, `__main__.py`, `manage.py` | Entrypoints |

Schemas and serializers map to the existing Domain layer; do not add a new
Data contracts layer.

### Go profile

Use only for `.go` files:

| Convention | Layer |
|---|---|
| `cmd/<application>` or `main.go` | Entrypoints |
| `handlers`, `http`, `api` | Transport / API |
| `service`, `services`, `usecase`, `usecases` | Services |
| `domain`, `model`, `models`, `entity`, `entities` | Domain |
| `repository`, `storage`, `database` | Persistence |
| `adapter`, `adapters`, `client`, `clients` | Infrastructure |
| `pkg` | Shared |
| `config` | Configuration |
| `testdata`, `*_test.go` | Tests |

The Go `internal` directory controls visibility and is not an architectural
role by itself. Continue scanning its descendant segments for a role; otherwise
fall back to its preserved directory hierarchy.

### Generic architecture conventions

When no universal or language-profile rule matches, use:

| Layer | Path segments |
|---|---|
| Persistence | `repository`, `repositories`, `persistence`, `database`, `db`, `storage`, `dao` |
| Transport / API | `api`, `route`, `routes`, `controller`, `controllers`, `handler`, `handlers`, `transport` |
| Presentation | `ui`, `component`, `components`, `page`, `pages`, `view`, `views`, `screen`, `screens` |
| Domain | `domain`, `model`, `models`, `entity`, `entities`, `aggregate`, `aggregates` |
| Services | `service`, `services`, `usecase`, `usecases`, `application` |
| Infrastructure | `adapter`, `adapters`, `provider`, `providers`, `integration`, `integrations`, `client`, `clients` |
| Shared | `shared`, `common`, `util`, `utils`, `helper`, `helpers`, `lib` |
| Entrypoints | `main.*`, `index.*`, `app.*`, `server.*`, or a file beneath `cmd` |
| Other | No rule matched |

Record the winning source as `universal`, `language-convention`, `generic`, or
`fallback`. Any recognized rule has `confidence: "inferred"`; `other` has
`confidence: "fallback"`. This metadata is not presented as a new UI badge.

Do not add framework-specific rules. Names such as `models`, `services`, and
`api` are architectural hints, not proof of the repository author's intent.

## Step 5 — Derive Domains or Features

Derive a domain only after project and layer classification:

1. Locate the path segment that matched the architectural layer.
2. Use the first directory segment after it as the domain.
3. When there is no directory after the matched segment, use `General`.
4. For entrypoint, configuration, test, tooling, and shared layers, use
   `General` instead of inferring a domain.
5. For an `other` layer, use the first preserved directory below the project
   boundary as the domain; use `General` only when no such directory exists.

Examples:

```text
backend/services/diagnostics/cpu.py  -> Services / diagnostics
backend/domain/users/model.py        -> Domain / users
frontend/components/account/Card.tsx -> Presentation / account
frontend/App.tsx                     -> Entrypoints / General
tests/users/test_service.py           -> Tests / General
engine/runtime/tasks/runner.py        -> Other / runtime
```

Domain names must come directly from normalized directory names. Do not rename,
singularize, summarize, or infer business concepts.

## Step 6 — Replace the Two-Directory Grouping With Three Levels

Extend the visible graph entity union:

```ts
type FileGraphEntity =
  | ArchitectureEntity
  | DomainEntity
  | FileEntity
```

Use collision-safe IDs:

```text
architecture:<project>:<layer>
domain:<project>:<layer>:<domain>
file:<repository-relative-path>
```

Default state returns architecture entities only. Each entity contains:

- project and architectural layer;
- represented file count;
- represented internal dependency count;
- diagnostic count; and
- whether it contains a search match.

Expanding an architecture entity replaces it with its domain entities.
Expanding a domain entity replaces it with its files.

To control clutter, use accordion behavior within each project:

- expanding an architecture peer collapses the previously expanded
  architecture peer in the same project;
- expanding a domain peer collapses the previously expanded domain peer under
  the same architecture entity; and
- expansion in a different project remains independent.

The existing expanded-items control above the canvas must show the active
architecture and domain path and allow either level to be collapsed.

## Step 7 — Aggregate Relationships at Visible Endpoints

For every original file edge:

1. Resolve the source and target file to their final visible entity.
2. Hide the edge when both files resolve to the same collapsed entity.
3. Group by the ordered final `(source, target)` IDs.
4. Count represented original edges as `dependencyCount`.
5. Preserve the original import direction.

This produces:

```text
Presentation -- 18 imports --> Services
```

When Services is expanded:

```text
Presentation -- 7 --> authentication
Presentation -- 8 --> diagnostics
Presentation -- 3 --> reporting
```

When diagnostics is expanded, only its endpoint changes to files. No analysis
job runs again during expansion.

## Step 8 — Classify Visible Arrows With Neutral Colors

Arrow color represents relationship context, not severity. Use this fixed,
low-saturation palette:

| Relationship | Color | Hex |
|---|---|---|
| Main application flow | Graphite | `#6F747B` |
| Cross-project dependency | Blue gray | `#7A8290` |
| Configuration/shared support | Warm gray | `#8B8278` |
| Test dependency | Sage gray | `#7D897F` |
| Other relationship | Cool gray | `#92969C` |
| Selected relationship | Dark graphite | `#3F444A` |

Determine the relationship class in this order:

1. Different projects -> cross-project.
2. Source or target is Tests -> test.
3. Source or target is Configuration or Shared -> support.
4. Both endpoints are part of Entrypoints, Presentation, Transport, Services,
   Domain, Persistence, or Infrastructure -> main flow.
5. Otherwise -> other.

Requirements:

- keep arrowheads the same color as their edge;
- retain aggregate count labels for counts greater than one;
- use the existing narrowly capped width variation for dependency volume;
- use solid arrows for every resolved dependency;
- do not use gradients, glow, animation, or saturated colors;
- do not interpret reverse-layer edges as violations;
- selected adjacency overrides relationship color with dark graphite; and
- preserve direction through arrowheads so meaning does not rely on color.

Add a compact edge legend only when at least two relationship classes are
visible. The legend must use a short line sample and text, not color alone.

## Step 9 — Order the Overall Layout

Assign a stable layout rank:

```text
Entrypoints    0
Presentation  1
Transport     2
Services      3
Domain        4
Persistence   5
Infrastructure 5
Other         3
```

Use the existing left-to-right layered layout for the main flow. Add only the
minimum layout metadata required to keep ranks stable.

Place supporting groups in separate lanes:

```text
Configuration  -> upper support lane
Shared         -> lower support lane
Tests          -> lower test lane
Tooling        -> outer lower lane
```

The exact coordinates remain the responsibility of the current layout helper.
Do not implement manual swimlane containers, nested React Flow parents, or a
new layout library. The implementation may provide rank and lane hints to the
existing ELK layout only.

## Step 10 — Preserve Search, Selection, and Hop Focus

- Search continues to match original repository-relative file paths.
- A collapsed architecture or domain entity remains visible and highlighted
  when it contains a matching file.
- Selecting an architecture or domain entity highlights its visible incoming
  and outgoing relationships without opening the file inspector.
- Selecting a file opens the existing inspector and resets focus to one hop.
- One-hop and two-hop traversal continue to operate over original file edges,
  treating incoming and outgoing files as adjacent for visibility.
- Neighboring collapsed groups remain collapsed; hop focus must not expand
  them automatically.
- Clearing selection restores all entities at the current expansion level.

Do not add new filtering, breadcrumbs, source previews, or inspector panels.
The existing expanded-items control is the only hierarchy-location indicator
in this implementation.

## Step 11 — Reset Architecture State for a New Repository

After successful repository analysis, reset only file-graph view state:

```text
selected architecture entity -> null
selected domain entity       -> null
selected file                -> null
expanded architecture        -> empty
expanded domain              -> empty
hop scope                    -> 1
search                       -> empty
positions                    -> empty
viewport                     -> null
```

Do not change package graph state or submit additional analysis jobs.

## Testing Plan

### Unit tests

- project boundaries follow the exact path-only rules;
- JavaScript/TypeScript, Python, and Go profiles are selected per file
  extension in a polyglot snapshot;
- universal conventions take precedence over language-specific candidates;
- every language-profile mapping returns the documented layer;
- Go `internal` remains structural rather than becoming a role;
- every file receives one layer and domain;
- classification precedence is deterministic;
- an unmatched nested file preserves its first directory as the project and
  its next directory as the `Other` domain;
- an unmatched top-level file falls back to
  `repository / Other / General`;
- architecture, domain, and file expansion produce correct visible endpoints;
- accordion behavior affects only peers within the same parent;
- aggregate counts equal represented original edges at every level;
- relationship classes follow their documented precedence;
- arrow classes do not alter dependency direction;
- search preserves a collapsed matching ancestor;
- one-hop and two-hop focus preserve required collapsed neighbors; and
- shuffled source collections produce identical visible graphs.

### Component tests

- architecture nodes show layer, project, file count, and diagnostics;
- domain nodes show domain, file count, and diagnostics;
- expand and collapse controls emit the correct IDs;
- selected edges use dark graphite independent of their base class;
- the edge legend appears only for two or more visible relationship classes;
  and
- file hop controls remain available only for a selected file.

### Browser tests

- Files initially shows architecture groups instead of directory modules;
- overall flow is ordered from entry/presentation toward persistence;
- expanding a layer reveals domains;
- expanding a domain reveals files;
- expanding a peer collapses its prior peer within the same project;
- selected file one-hop and two-hop behavior remains intact;
- search finds a file through its collapsed ancestors;
- aggregate edge counts remain stable while drilling down; and
- loading another repository restores the architecture overview.

## Acceptance Criteria

The implementation is complete when:

1. The default Files canvas displays architecture entities only.
2. Every file deterministically belongs to one project, layer, and domain,
   using its language convention profile when applicable.
3. Users can drill from architecture to domain to individual files.
4. The canvas never invents, reverses, or drops a represented dependency.
5. Aggregate edge counts remain correct at each visible level.
6. Main, cross-project, support, test, and other relationships use the defined
   neutral arrow treatment.
7. Layout communicates the main application flow and separates supporting
   concerns without a new layout dependency.
8. Existing search, diagnostics, inspector, minimap, and hop focus continue to
   work.
9. The analyzers, queue, workers, API contract, and package graph are unchanged.
10. Unit, component, browser, lint, typecheck, and production-build checks
    pass.

## Recommended Implementation Order

1. Add and test universal, JavaScript/TypeScript, Python, Go, and fallback
   path classification.
2. Extend the pure visible-graph transformation for architecture and domains.
3. Add architecture and domain nodes with scoped expansion state.
4. Add endpoint aggregation and neutral relationship classes.
5. Add layout rank and support-lane hints through the existing layout helper.
6. Preserve search, selection, hop focus, reset, minimap, and position behavior.
7. Update component and browser regressions.
8. Run the complete frontend verification suite.
