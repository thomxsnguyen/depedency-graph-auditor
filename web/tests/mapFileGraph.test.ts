import {
  diagnosticsForFile,
  fileGraphCounts,
  incomingFiles,
  outgoingFiles,
} from "../src/graph/fileGraphSelectors"
import { fileNodeId, mapFileGraph } from "../src/graph/mapFileGraph"
import { classifyFile } from "../src/graph/fileCategory"
import { buildHierarchicalFileGraph, moduleEntityId } from "../src/graph/hierarchicalFileGraph"
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

describe("file graph mapping", () => {
  it("uses complete paths as stable identities and preserves edge direction", () => {
    const visible = buildHierarchicalFileGraph(snapshot, new Set(["src", "pages"]), null, 1)
    const graph = mapFileGraph(visible, null, null, "", {})
    expect(fileNodeId("src/Button.tsx")).not.toBe(fileNodeId("pages/Button.tsx"))
    expect(graph.nodes.map((node) => node.id)).toContain(fileNodeId("src/Button.tsx"))
    expect(graph.edges.find((edge) => edge.source === fileNodeId("src/App.tsx"))).toMatchObject({
      source: fileNodeId("src/App.tsx"),
      target: fileNodeId("src/Button.tsx"),
    })
  })

  it("groups diagnostics and highlights only resolved adjacent edges", () => {
    const visible = buildHierarchicalFileGraph(snapshot, new Set(["src", "pages"]), "src/App.tsx", "all", "src/app")
    const graph = mapFileGraph(visible, "src/App.tsx", null, "src/app", {})
    expect(graph.nodes.find((node) => node.data.path === "src/App.tsx")?.data.diagnosticCount).toBe(1)
    expect(graph.edges.filter((edge) => edge.className?.includes("selected"))).toHaveLength(1)
    expect(graph.edges).toHaveLength(snapshot.edges.length)
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

  it("adds the category to each mapped node", () => {
    const visible = buildHierarchicalFileGraph(snapshot, new Set(["src", "pages"]), null, 1)
    const graph = mapFileGraph(visible, null, null, "", {})
    expect(graph.nodes.find((node) => node.data.path === "pages/Button.tsx")?.data.category).toBe("frontend")
  })

  it("maps collapsed modules and aggregate edge labels", () => {
    const visible = buildHierarchicalFileGraph(snapshot, new Set(), null, 1)
    const graph = mapFileGraph({
      ...visible,
      edges: visible.edges.map((edge) => ({ ...edge, dependencyCount: 2 })),
    }, null, "src", "", {})
    expect(graph.nodes.find((node) => node.id === moduleEntityId("src"))).toMatchObject({
      type: "module",
      selected: true,
    })
    expect(graph.edges[0]?.label).toBe("2")
  })

  it("selects sorted incoming, outgoing, diagnostic, and summary data", () => {
    expect(incomingFiles(snapshot, "src/Button.tsx")).toEqual(["pages/Button.tsx", "src/App.tsx"])
    expect(outgoingFiles(snapshot, "src/App.tsx")).toEqual(["src/Button.tsx"])
    expect(diagnosticsForFile(snapshot, "src/App.tsx")).toHaveLength(1)
    expect(fileGraphCounts(snapshot)).toEqual({ files: 3, imports: 2, diagnostics: 1 })
  })
})
