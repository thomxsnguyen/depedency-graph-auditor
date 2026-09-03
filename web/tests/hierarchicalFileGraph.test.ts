import {
  buildHierarchicalFileGraph,
  fileEntityId,
  moduleEntityId,
  modulePathForFile,
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

describe("hierarchical file graph", () => {
  it("assigns files using the deterministic two-directory rule", () => {
    expect(modulePathForFile("main.go")).toBe(".")
    expect(modulePathForFile("config/settings.py")).toBe("config")
    expect(modulePathForFile("frontend/App.tsx")).toBe("frontend")
    expect(modulePathForFile("frontend/components/Button.tsx")).toBe("frontend/components")
    expect(modulePathForFile("backend\\services\\payments\\charge.py")).toBe("backend/services")
  })

  it("summarizes collapsed modules and aggregates directed edges", () => {
    const graph = buildHierarchicalFileGraph(snapshot, new Set(), null, 1)
    const components = graph.nodes.find((node) => node.id === moduleEntityId("frontend/components"))
    expect(components).toMatchObject({
      kind: "module",
      fileCount: 2,
      internalDependencyCount: 1,
      diagnosticCount: 1,
    })
    expect(graph.edges.find((edge) => (
      edge.from === moduleEntityId("frontend/pages")
      && edge.to === moduleEntityId("frontend/components")
    ))?.dependencyCount).toBe(2)
    expect(graph.edges.some((edge) => edge.from === edge.to)).toBe(false)
  })

  it("maps edges to files or modules according to expansion state", () => {
    const oneExpanded = buildHierarchicalFileGraph(snapshot, new Set(["frontend/components"]), null, 1)
    expect(oneExpanded.nodes.some((node) => node.id === fileEntityId("frontend/components/Button.tsx"))).toBe(true)
    expect(oneExpanded.nodes.some((node) => node.id === moduleEntityId("frontend/components"))).toBe(false)
    expect(oneExpanded.edges).toEqual(expect.arrayContaining([
      expect.objectContaining({
        from: fileEntityId("frontend/components/Button.tsx"),
        to: moduleEntityId("backend/services"),
      }),
      expect.objectContaining({
        from: moduleEntityId("frontend/pages"),
        to: fileEntityId("frontend/components/Button.tsx"),
      }),
    ]))

    const bothExpanded = buildHierarchicalFileGraph(
      snapshot,
      new Set(["frontend/components", "backend/services"]),
      null,
      1,
    )
    expect(bothExpanded.edges).toContainEqual(expect.objectContaining({
      from: fileEntityId("frontend/components/Button.tsx"),
      to: fileEntityId("backend/services/data.py"),
    }))
  })

  it("limits selected files to one or two undirected dependency hops", () => {
    const allModules = new Set(snapshot.nodes.map((node) => modulePathForFile(node.path)))
    const selected = "frontend/components/Button.tsx"
    const oneHop = buildHierarchicalFileGraph(snapshot, allModules, selected, 1)
    const twoHops = buildHierarchicalFileGraph(snapshot, allModules, selected, 2)
    const all = buildHierarchicalFileGraph(snapshot, allModules, selected, "all")

    expect(oneHop.nodes.map((node) => node.path).sort()).toEqual([
      "backend/services/data.py",
      "frontend/components/Button.tsx",
      "frontend/components/Card.tsx",
      "frontend/pages/Home.tsx",
    ])
    expect(twoHops.nodes.map((node) => node.path)).toContain("backend/services/cache.py")
    expect(all.nodes).toHaveLength(snapshot.nodes.length)
  })

  it("retains a matching collapsed module outside hop focus", () => {
    const graph = buildHierarchicalFileGraph(
      snapshot,
      new Set(["frontend/components"]),
      "frontend/components/Button.tsx",
      1,
      "config/settings.py",
    )
    expect(graph.nodes).toContainEqual(expect.objectContaining({
      id: moduleEntityId("config"),
      searchMatch: true,
    }))
  })

  it("is deterministic when snapshot collections are shuffled", () => {
    const shuffled: FileGraphSnapshot = {
      ...snapshot,
      nodes: [...snapshot.nodes].reverse(),
      edges: [...snapshot.edges].reverse(),
      diagnostics: [...snapshot.diagnostics].reverse(),
    }
    expect(buildHierarchicalFileGraph(shuffled, new Set(), null, 1))
      .toEqual(buildHierarchicalFileGraph(snapshot, new Set(), null, 1))
  })
})
