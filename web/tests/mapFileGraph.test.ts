import {
  diagnosticsForFile,
  fileGraphCounts,
  incomingFiles,
  outgoingFiles,
} from "../src/graph/fileGraphSelectors"
import { fileNodeId, mapFileGraph } from "../src/graph/mapFileGraph"
import { classifyFile } from "../src/graph/fileCategory"
import {
  architectureEntityId,
  architectureForFile,
  buildHierarchicalFileGraph,
  domainEntityId,
} from "../src/graph/hierarchicalFileGraph"
import type { FileGraphSnapshot } from "../src/types/fileGraph"

const snapshot: FileGraphSnapshot = {
  schemaVersion: 1,
  root: "demo",
  nodes: [
    { path: "src/Button.tsx" },
    { path: "pages/Button.tsx" },
    { path: "src/App.tsx" },
  ],
  edges: [
    { from: "src/App.tsx", to: "src/Button.tsx" },
    { from: "pages/Button.tsx", to: "src/Button.tsx" },
  ],
  diagnostics: [
    { path: "src/App.tsx", import: "./missing", message: "unresolved local import" },
  ],
}

function fullyExpandedGraph(selectedPath: string | null = null, search = "") {
  const architecture = snapshot.nodes.map((node) => architectureForFile(node.path))
  return buildHierarchicalFileGraph(
    snapshot,
    new Set(architecture.map((item) => architectureEntityId(item.project, item.layer))),
    new Set(architecture.map((item) => domainEntityId(item.project, item.layer, item.domain))),
    selectedPath,
    "all",
    search,
  )
}

describe("file graph mapping", () => {
  it("uses complete paths as stable identities and preserves edge direction", () => {
    const graph = mapFileGraph(fullyExpandedGraph(), null, null, "", {}, "all")
    expect(fileNodeId("src/Button.tsx")).not.toBe(fileNodeId("pages/Button.tsx"))
    expect(graph.nodes.map((node) => node.id)).toContain(fileNodeId("src/Button.tsx"))
    expect(graph.edges).toContainEqual(expect.objectContaining({
      source: architectureEntityId("src", "entrypoint"),
      target: architectureEntityId("src", "other"),
    }))
  })

  it("groups diagnostics and highlights only resolved adjacent edges", () => {
    const graph = mapFileGraph(
      fullyExpandedGraph("src/App.tsx", "src/app"),
      "src/App.tsx",
      null,
      "src/app",
      {},
    )
    expect(graph.nodes.find((node) => node.data.path === "src/App.tsx")?.data.diagnosticCount).toBe(1)
    expect(graph.edges.filter((edge) => edge.className?.includes("selected"))).toHaveLength(1)
  })

  it("classifies file roles with specific rules taking precedence over folders", () => {
    expect(classifyFile("frontend/components/Button.tsx")).toBe("frontend")
    expect(classifyFile("frontend/components/Button.test.tsx")).toBe("test")
    expect(classifyFile("src/config/settings.py")).toBe("configuration")
    expect(classifyFile("src/generated/client.ts")).toBe("generated")
    expect(classifyFile("scripts/report.py")).toBe("script")
    expect(classifyFile("src/domain/model.go")).toBe("application")
    expect(classifyFile("main.go")).toBe("general")
  })

  it("adds category and architecture metadata to mapped file nodes", () => {
    const graph = mapFileGraph(fullyExpandedGraph(), null, null, "", {})
    const fileNode = graph.nodes.find((node) => node.data.path === "pages/Button.tsx")
    expect(fileNode?.data).toMatchObject({
      category: "frontend",
      project: "pages",
      layer: "presentation",
    })
    expect(fileNode).toMatchObject({
      parentId: domainEntityId("pages", "presentation", "General"),
      draggable: false,
    })
    expect(graph.nodes.find((node) => node.id === domainEntityId("pages", "presentation", "General"))).toMatchObject({
      parentId: architectureEntityId("pages", "presentation"),
      draggable: false,
    })
  })

  it("maps architecture groups and neutral aggregate arrows", () => {
    const visible = buildHierarchicalFileGraph(snapshot, new Set(), new Set(), null, 1)
    const selectedId = architectureEntityId("src", "entrypoint")
    const graph = mapFileGraph({
      ...visible,
      edges: visible.edges.map((edge) => ({ ...edge, dependencyCount: 2 })),
    }, null, selectedId, "", {}, "all")
    expect(graph.nodes.find((node) => node.id === selectedId)).toMatchObject({
      type: "architecture",
      selected: true,
    })
    const focusedEdge = graph.edges.find((edge) => edge.className?.includes("selected"))!
    const sourceEdge = visible.edges.find((edge) => edge.id === focusedEdge.id)!
    expect(focusedEdge).toMatchObject({
      label: "2 imports",
      markerEnd: expect.objectContaining({ color: expect.any(String) }),
    })
    expect(focusedEdge.sourceHandle).toBe(`source-${sourceEdge.relationship}`)
    expect(focusedEdge.targetHandle).toBe(`target-${sourceEdge.relationship}`)
  })

  it("hides connections until focused and supports the complete view", () => {
    const visible = buildHierarchicalFileGraph(snapshot, new Set(), new Set(), null, 1)
    const focused = mapFileGraph(visible, null, null, "", {}, "focused")
    const complete = mapFileGraph(visible, null, null, "", {}, "all")
    expect(focused.edges).toHaveLength(0)
    expect(complete.edges).toHaveLength(visible.edges.length)
    expect(complete.edges.every((edge) => edge.style?.opacity === 0.42)).toBe(true)
  })

  it("selects sorted incoming, outgoing, diagnostic, and summary data", () => {
    expect(incomingFiles(snapshot, "src/Button.tsx")).toEqual(["pages/Button.tsx", "src/App.tsx"])
    expect(outgoingFiles(snapshot, "src/App.tsx")).toEqual(["src/Button.tsx"])
    expect(diagnosticsForFile(snapshot, "src/App.tsx")).toHaveLength(1)
    expect(fileGraphCounts(snapshot)).toEqual({ files: 3, imports: 2, diagnostics: 1 })
  })
})
