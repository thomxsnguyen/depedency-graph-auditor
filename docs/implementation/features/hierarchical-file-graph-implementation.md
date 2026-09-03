# Hierarchical File Graph — Implementation Plan

## Status

Implemented according to the scoped plan below.

## Objective

Make large file-dependency graphs understandable by presenting a repository as
collapsed folder/module nodes first, then revealing individual files only when
the user expands a module. Repeated relationships are aggregated, diagnostics
are summarized, and selecting a file can limit the canvas to one or two
dependency hops.

The existing normalized `schemaVersion: 1` file graph remains the source of
truth.

## Hard Scope Boundary

This implementation includes only:

- a deterministic, language-neutral path-to-module rule;
- a pure frontend transformation from the existing file graph to a visible
  hierarchical graph;
- collapsed module nodes with file, dependency, and diagnostic counts;
- aggregate edges between visible modules or files;
- expand and collapse behavior for individual modules;
- one-hop and two-hop focus for a selected file;
- layout and selection updates required by the changing visible graph; and
- focused unit, component, and browser tests.

It does not include:

- analyzer, discovery, resolver, queue, worker, retry, or DLQ changes;
- API or `filegraph.Report` schema changes;
- database storage or migrations;
- new language support;
- package-dependency graph changes or combined package/file graphs;
- symbol, function, class, call, data-flow, or runtime analysis;
- architectural rule enforcement or layer-violation detection;
- user-defined module rules, repository configuration, or framework-specific
  module detection;
- source-code browsing or editing;
- graph virtualization, server-side aggregation, caching, or pagination;
- redesign of the surrounding UI; or
- deployment changes.

## Existing Behavior to Preserve

- Local and GitHub file analysis continue to return the current versioned
  normalized file graph.
- JavaScript/TypeScript, Python, and Go analysis remain unchanged.
- File analysis continues to use its current in-memory queue and workers.
- Search, file selection, the file inspector, diagnostics, minimap, zoom,
  fit-to-view, and reset-layout behavior remain available.
- File category colors remain presentation-only and continue to be derived from
  file paths.
- Package dependency mode remains unchanged and remains the application's
  initial graph mode.
- Loading a different repository clears file selection, viewport, positions,
  expansion state, and hop focus before displaying the new graph.

## Architecture

```text
existing FileGraphSnapshot
          |
          v
pure module index and aggregation
          |
          v
expanded module state + optional hop focus
          |
          v
VisibleFileGraph
  - module nodes
  - file nodes for expanded modules
  - counted visible edges
          |
          v
existing React Flow canvas and layout
```

The transformation belongs in the frontend graph layer. It is a view of the
existing complete report and must not modify or discard the normalized source
snapshot.

## Files to Add

```text
web/src/graph/hierarchicalFileGraph.ts
web/src/components/FileGraphCanvas/ModuleNode.tsx
web/tests/hierarchicalFileGraph.test.ts
```

## Files to Change

```text
web/src/app/AuditWorkspace.tsx
web/src/components/FileGraphCanvas/FileGraphCanvas.tsx
web/src/components/FileGraphCanvas/FileNode.tsx       only if shared node data requires it
web/src/graph/mapFileGraph.ts
web/src/styles/globals.css
web/tests/components.test.tsx
web/tests/browser/audit-studio.spec.ts
```

Do not modify Go source, HTTP handlers, file-graph data sources, JSON fixtures,
package-graph components, queue code, or deployment files.

## Data Model

Define frontend-only view types. They must not be added to
`FileGraphSnapshot` or emitted by the backend.

```ts
type FileGraphEntity =
  | {
      kind: "module"
      id: string
      path: string
      fileCount: number
      internalDependencyCount: number
      diagnosticCount: number
      expanded: false
    }
  | {
      kind: "file"
      id: string
      path: string
      modulePath: string
      diagnosticCount: number
    }

interface VisibleFileGraphEdge {
  id: string
  from: string
  to: string
  dependencyCount: number
}

interface VisibleFileGraph {
  nodes: FileGraphEntity[]
  edges: VisibleFileGraphEdge[]
}

type DependencyHopScope = 1 | 2 | "all"
```

IDs must include their entity kind so a module path cannot collide with a file
path:

```text
module:frontend/components
file:frontend/components/Button.tsx
```

All returned nodes and edges must be sorted deterministically before mapping to
React Flow.

## Step 1 — Assign Files to Modules

Add a pure `modulePathForFile(path)` function using normalized slash-separated
paths.

Use this fixed rule:

1. Normalize separators and remove empty path segments.
2. A top-level file belongs to the synthetic module `.`.
3. A file below one directory belongs to that directory.
4. A file below two or more directories belongs to its first two directory
   segments.

Examples:

```text
main.go                              -> .
config/settings.py                   -> config
frontend/App.tsx                     -> frontend
frontend/components/Button.tsx       -> frontend/components
backend/services/payments/charge.py  -> backend/services
```

The two-segment limit is deliberate: it provides stable architectural groups
without introducing language or framework assumptions. Do not inspect file
contents, manifests, package declarations, import aliases, or build files when
assigning a module.

Build one immutable lookup from file path to module path for each loaded
snapshot. Every source node must belong to exactly one module.

### Step 1 tests

- top-level, one-directory, and deeply nested paths map correctly;
- Windows separators normalize correctly;
- files with the same basename retain distinct identities; and
- assignment is deterministic regardless of input node order.

## Step 2 — Build the Collapsed Module Graph

When no modules are expanded, return one node per module.

For every module calculate:

- total assigned files;
- number of original edges whose two endpoints are inside the module; and
- total diagnostics attached to its assigned files.

Transform each original file edge as follows:

1. Map its source and target files to their modules.
2. Hide the edge when both files belong to the same collapsed module.
3. Otherwise aggregate it by the ordered `(sourceModule, targetModule)` pair.
4. Store the number of original file edges as `dependencyCount`.

Do not infer edges that do not exist in the source snapshot. Preserve edge
direction.

## Step 3 — Expand and Collapse Modules

Keep `expandedModulePaths` as UI state owned by `AuditWorkspace`. It is not
persisted to local storage in this implementation.

Use an explicit expand/collapse control on each module node. A normal node
click selects or focuses the node; it must not unexpectedly expand the graph.

For an expanded module:

- replace its collapsed module node with all files assigned to it;
- render original file-to-file edges when both endpoint modules are expanded;
- render and aggregate file-to-module edges when only the source module is
  expanded;
- render and aggregate module-to-file edges when only the target module is
  expanded; and
- render internal file-to-file edges within the expanded module.

The transformation must group by the final visible endpoint IDs. This prevents
parallel duplicate lines while retaining the exact number of represented file
dependencies.

Collapsing a module must:

- remove its file nodes from the visible graph;
- restore its summary module node and aggregated edges;
- clear the selected file if that file is inside the collapsed module; and
- remove obsolete visible-node positions before the next layout.

Do not add "expand all" in this implementation. It would recreate the crowded
default and is not required for module-by-module exploration.

## Step 4 — Add the Module Node

Add `ModuleNode.tsx` as a read-only graph entity with:

- folder icon and normalized module path;
- file count;
- internal dependency count;
- diagnostic count when nonzero; and
- one clearly labeled `Expand module` control.

Use the current neutral, sharp-corner visual system. Module nodes should have
slightly stronger visual weight than file nodes but must not introduce vibrant
colors, gradients, decorative chrome, or additional design tokens unrelated to
the file graph.

Aggregate visible edges display their count only when greater than one. Edge
width may vary within a narrow bounded range; it must not scale without a cap.
Selected adjacency uses the existing emphasis treatment.

## Step 5 — Focus a Selected File by Dependency Hops

Hop focus applies only when an individual file is selected.

Calculate neighbors from the original directed file edges, treating incoming
and outgoing relationships as adjacent for visibility:

- `1` shows the selected file and direct incoming/outgoing neighbors;
- `2` additionally shows neighbors one more edge away; and
- `all` disables hop filtering for currently expanded modules.

Default to one hop when a file is selected. Provide a compact control with
`1 hop`, `2 hops`, and `All`, visible only while a file is selected.

Filtering occurs after hierarchical endpoint transformation. Preserve any
collapsed module node needed to represent a visible cross-module relationship.
Do not automatically expand neighboring modules.

Clearing selection removes hop filtering. Changing the selected file resets
the scope to one hop.

## Step 6 — Integrate Layout and Existing Interactions

Update `mapFileGraph` and `FileGraphCanvas` to accept `VisibleFileGraph` rather
than mapping the complete snapshot directly.

Requirements:

- run layout only against currently visible nodes and edges;
- fit the graph after expansion, collapse, or hop-scope changes;
- keep manually positioned nodes when their visible IDs still exist;
- discard positions for nodes that no longer exist;
- keep search matched against original file paths;
- when a search matches a file in a collapsed module, keep its containing
  module visible and mark that module as containing a match;
- preserve file inspector data from the original snapshot; and
- keep the minimap synchronized with the visible graph.

The existing reset-layout control resets positions and the viewport. It must
not change expanded modules or the selected hop scope.

## Step 7 — Reset State for a New Repository

After a successful GitHub repository analysis, reset only file-graph view
state:

```text
selected file       -> null
expanded modules    -> empty set
hop scope           -> 1
search              -> empty
positions           -> empty
viewport            -> null
```

Do not change package-graph state or trigger another backend analysis.

## Testing Plan

### Unit tests

- module assignment follows the exact two-segment rule;
- collapsed modules contain correct file and diagnostic counts;
- internal dependencies are counted but not rendered as collapsed self-edges;
- repeated cross-module edges aggregate with correct direction and count;
- expanding neither, one, or both endpoint modules produces correct visible
  endpoint types;
- one-hop and two-hop traversal include incoming and outgoing neighbors;
- unrelated nodes are excluded by hop focus;
- transformation output remains deterministic after shuffled input; and
- search retains the containing collapsed module.

### Component tests

- module counts and diagnostics render;
- expand and collapse callbacks receive the correct module path;
- hop controls appear only for a selected file; and
- the active hop scope is exposed accessibly.

### Browser tests

- Files initially displays collapsed modules instead of every file;
- expanding a module reveals its files without changing analysis data;
- collapsing it restores the aggregate node and edge counts;
- selecting a file defaults to one-hop focus;
- switching to two hops reveals the expected additional nodes; and
- loading another repository restores the collapsed default.

## Acceptance Criteria

The implementation is complete when:

1. The Files canvas defaults to deterministic module nodes.
2. Every analyzed file belongs to exactly one module.
3. Aggregate edge counts equal the original relationships represented by each
   visible edge.
4. Expanding and collapsing modules never changes the normalized source graph.
5. File selection can be limited to one or two dependency hops.
6. Diagnostics remain traceable from module summaries to individual files.
7. Existing local and GitHub file analysis requires no backend change.
8. Package dependency behavior is unchanged.
9. Unit, component, browser, lint, typecheck, and production build checks pass.

## Recommended Implementation Order

1. Add module assignment and collapsed aggregation with unit tests.
2. Add expanded endpoint transformation with unit tests.
3. Add `ModuleNode` and connect expansion state.
4. Add one-hop and two-hop filtering.
5. Adapt layout, search, selection, minimap, and reset behavior.
6. Add component and browser regression coverage.
7. Run the complete frontend verification suite.
