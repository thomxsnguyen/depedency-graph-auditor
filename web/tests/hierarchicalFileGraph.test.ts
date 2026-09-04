import {
  architectureEntityId,
  architectureForFile,
  buildHierarchicalFileGraph,
  domainEntityId,
  expandedArchitectureItems,
  fileEntityId,
} from "../src/graph/hierarchicalFileGraph"
import type { FileGraphSnapshot } from "../src/types/fileGraph"

const snapshot: FileGraphSnapshot = {
  schemaVersion: 1,
  root: "large-repository",
  nodes: [
    { path: "backend/api/route.py" },
    { path: "backend/services/cache.py" },
    { path: "backend/services/data.py" },
    { path: "config/settings.py" },
    { path: "frontend/components/Button.tsx" },
    { path: "frontend/components/Card.tsx" },
    { path: "frontend/pages/Home.tsx" },
  ],
  edges: [
    { from: "frontend/components/Button.tsx", to: "frontend/components/Card.tsx" },
    { from: "frontend/pages/Home.tsx", to: "frontend/components/Button.tsx" },
    { from: "frontend/pages/Home.tsx", to: "frontend/components/Card.tsx" },
    { from: "frontend/components/Button.tsx", to: "backend/services/data.py" },
    { from: "frontend/components/Card.tsx", to: "backend/services/data.py" },
    { from: "backend/services/data.py", to: "backend/services/cache.py" },
    { from: "backend/services/cache.py", to: "backend/api/route.py" },
  ],
  diagnostics: [
    { path: "frontend/components/Button.tsx", message: "missing import" },
    { path: "backend/services/data.py", message: "missing import" },
  ],
}

const frontendArchitectureId = architectureEntityId("frontend", "presentation")
const frontendDomainId = domainEntityId("frontend", "presentation", "General")
const backendServicesId = architectureEntityId("backend", "application")

function allExpansionIds(graph: FileGraphSnapshot) {
  const architectures = graph.nodes.map((node) => architectureForFile(node.path))
  return {
    architectureIds: new Set(architectures.map((item) => architectureEntityId(item.project, item.layer))),
    domainIds: new Set(architectures.map((item) => domainEntityId(item.project, item.layer, item.domain))),
  }
}

describe("hierarchical file graph", () => {
  it("summarizes architecture groups and aggregates directed edges", () => {
    const graph = buildHierarchicalFileGraph(snapshot, new Set(), new Set(), null, 1)
    expect(graph.nodes.find((node) => node.id === frontendArchitectureId)).toMatchObject({
      kind: "architecture",
      project: "frontend",
      layer: "presentation",
      fileCount: 3,
      internalDependencyCount: 3,
      diagnosticCount: 1,
    })
    expect(graph.edges.find((edge) => edge.from === frontendArchitectureId && edge.to === backendServicesId)).toMatchObject({
      dependencyCount: 2,
      relationship: "cross-project",
    })
    expect(graph.edges.some((edge) => edge.from === edge.to)).toBe(false)
  })

  it("drills from architecture to domains and then files", () => {
    const domains = buildHierarchicalFileGraph(snapshot, new Set([frontendArchitectureId]), new Set(), null, 1)
    expect(domains.nodes).toContainEqual(expect.objectContaining({
      id: frontendDomainId,
      kind: "domain",
      fileCount: 3,
    }))
    expect(domains.nodes).toContainEqual(expect.objectContaining({
      id: frontendArchitectureId,
      expanded: true,
    }))

    const files = buildHierarchicalFileGraph(
      snapshot,
      new Set([frontendArchitectureId]),
      new Set([frontendDomainId]),
      null,
      1,
    )
    expect(files.nodes).toContainEqual(expect.objectContaining({
      id: fileEntityId("frontend/components/Button.tsx"),
      kind: "file",
    }))
    expect(files.edges).toContainEqual(expect.objectContaining({
      from: fileEntityId("frontend/components/Button.tsx"),
      to: fileEntityId("frontend/components/Card.tsx"),
      relationship: "main",
    }))
    expect(files.edges).toContainEqual(expect.objectContaining({
      from: frontendArchitectureId,
      to: backendServicesId,
      dependencyCount: 2,
      relationship: "cross-project",
    }))
  })

  it("classifies main, support, test, and other relationships without changing direction", () => {
    const relationships: FileGraphSnapshot = {
      schemaVersion: 1,
      root: "relationships",
      nodes: [
        { path: "backend/api/route.py" },
        { path: "backend/services/users.py" },
        { path: "backend/config/settings.py" },
        { path: "backend/tests/test_users.py" },
        { path: "engine/scripts/task.py" },
        { path: "engine/unknown/work.py" },
      ],
      edges: [
        { from: "backend/api/route.py", to: "backend/services/users.py" },
        { from: "backend/config/settings.py", to: "backend/services/users.py" },
        { from: "backend/tests/test_users.py", to: "backend/services/users.py" },
        { from: "engine/scripts/task.py", to: "engine/unknown/work.py" },
      ],
      diagnostics: [],
    }
    const graph = buildHierarchicalFileGraph(relationships, new Set(), new Set(), null, 1)
    expect(new Set(graph.edges.map((edge) => edge.relationship))).toEqual(new Set(["main", "support", "test", "other"]))
    expect(graph.edges).toContainEqual(expect.objectContaining({
      from: architectureEntityId("backend", "transport"),
      to: architectureEntityId("backend", "application"),
    }))
  })

  it("limits selected files to one or two undirected dependency hops", () => {
    const { architectureIds, domainIds } = allExpansionIds(snapshot)
    const selected = "frontend/components/Button.tsx"
    const oneHop = buildHierarchicalFileGraph(snapshot, architectureIds, domainIds, selected, 1)
    const twoHops = buildHierarchicalFileGraph(snapshot, architectureIds, domainIds, selected, 2)
    const all = buildHierarchicalFileGraph(snapshot, architectureIds, domainIds, selected, "all")

    expect(oneHop.nodes.flatMap((node) => node.kind === "file" ? [node.path] : []).sort()).toEqual([
      "backend/services/data.py",
      "frontend/components/Button.tsx",
      "frontend/components/Card.tsx",
      "frontend/pages/Home.tsx",
    ])
    expect(twoHops.nodes).toContainEqual(expect.objectContaining({ path: "backend/services/cache.py" }))
    expect(all.nodes.filter((node) => node.kind === "file")).toHaveLength(snapshot.nodes.length)
  })

  it("retains a matching collapsed architecture group outside hop focus", () => {
    const graph = buildHierarchicalFileGraph(
      snapshot,
      new Set([frontendArchitectureId]),
      new Set([frontendDomainId]),
      "frontend/components/Button.tsx",
      1,
      "config/settings.py",
    )
    expect(graph.nodes).toContainEqual(expect.objectContaining({
      id: architectureEntityId("config", "configuration"),
      searchMatch: true,
    }))
  })

  it("describes active architecture and domain expansion", () => {
    expect(expandedArchitectureItems(
      snapshot,
      new Set([frontendArchitectureId]),
      new Set([frontendDomainId]),
    )).toEqual([
      { kind: "architecture", id: frontendArchitectureId, label: "frontend / Presentation" },
      { kind: "domain", id: frontendDomainId, label: "frontend / Presentation / General" },
    ])
  })

  it("is deterministic when snapshot collections are shuffled", () => {
    const shuffled: FileGraphSnapshot = {
      ...snapshot,
      nodes: [...snapshot.nodes].reverse(),
      edges: [...snapshot.edges].reverse(),
      diagnostics: [...snapshot.diagnostics].reverse(),
    }
    expect(buildHierarchicalFileGraph(shuffled, new Set(), new Set(), null, 1))
      .toEqual(buildHierarchicalFileGraph(snapshot, new Set(), new Set(), null, 1))
  })
})
