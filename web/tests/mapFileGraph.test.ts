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
    const graph = mapFileGraph(fullyExpandedGraph(), null, null, "", {})
    expect(fileNodeId("src/Button.tsx")).not.toBe(fileNodeId("pages/Button.tsx"))
    expect(graph.nodes.map((node) => node.id)).toContain(fileNodeId("src/Button.tsx"))
    expect(graph.edges.find((edge) => edge.source === fileNodeId("src/App.tsx"))).toMatchObject({
      source: fileNodeId("src/App.tsx"),
      target: fileNodeId("src/Button.tsx"),
    })
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
    expect(graph.nodes.find((node) => node.data.path === "pages/Button.tsx")?.data).toMatchObject({
      category: "frontend",
      project: "pages",
      layer: "presentation",
    })
  })

  it("maps architecture groups and neutral aggregate arrows", () => {
    const visible = buildHierarchicalFileGraph(snapshot, new Set(), new Set(), null, 1)
    const selectedId = architectureEntityId("src", "entrypoint")
    const graph = mapFileGraph({
      ...visible,
      edges: visible.edges.map((edge) => ({ ...edge, dependencyCount: 2 })),
    }, null, selectedId, "", {})
    expect(graph.nodes.find((node) => node.id === selectedId)).toMatchObject({
      type: "architecture",
      selected: true,
    })
    expect(graph.edges[0]).toMatchObject({
      label: "2",
      markerEnd: expect.objectContaining({ color: expect.any(String) }),
    })
  })

  it("selects sorted incoming, outgoing, diagnostic, and summary data", () => {
    expect(incomingFiles(snapshot, "src/Button.tsx")).toEqual(["pages/Button.tsx", "src/App.tsx"])
    expect(outgoingFiles(snapshot, "src/App.tsx")).toEqual(["src/Button.tsx"])
    expect(diagnosticsForFile(snapshot, "src/App.tsx")).toHaveLength(1)
    expect(fileGraphCounts(snapshot)).toEqual({ files: 3, imports: 2, diagnostics: 1 })
  })
})
