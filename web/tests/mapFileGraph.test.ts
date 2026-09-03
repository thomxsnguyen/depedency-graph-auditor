import {
  diagnosticsForFile,
  fileGraphCounts,
  incomingFiles,
  outgoingFiles,
} from "../src/graph/fileGraphSelectors"
import { fileEdgeId, fileNodeId, mapFileGraph } from "../src/graph/mapFileGraph"
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
    const graph = mapFileGraph(snapshot, null, "", {})
    expect(fileNodeId("src/Button.tsx")).not.toBe(fileNodeId("pages/Button.tsx"))
    expect(graph.nodes.map((node) => node.id)).toContain(fileNodeId("src/Button.tsx"))
    expect(graph.edges[0]).toMatchObject({
      id: fileEdgeId("src/App.tsx", "src/Button.tsx"),
      source: fileNodeId("src/App.tsx"),
      target: fileNodeId("src/Button.tsx"),
    })
  })

  it("groups diagnostics and highlights only resolved adjacent edges", () => {
    const graph = mapFileGraph(snapshot, "src/App.tsx", "src/app", {})
    expect(graph.nodes.find((node) => node.data.path === "src/App.tsx")?.data.diagnosticCount).toBe(1)
    expect(graph.edges.filter((edge) => edge.className?.includes("selected"))).toHaveLength(1)
    expect(graph.edges).toHaveLength(snapshot.edges.length)
  })

  it("selects sorted incoming, outgoing, diagnostic, and summary data", () => {
    expect(incomingFiles(snapshot, "src/Button.tsx")).toEqual(["pages/Button.tsx", "src/App.tsx"])
    expect(outgoingFiles(snapshot, "src/App.tsx")).toEqual(["src/Button.tsx"])
    expect(diagnosticsForFile(snapshot, "src/App.tsx")).toHaveLength(1)
    expect(fileGraphCounts(snapshot)).toEqual({ files: 3, imports: 2, diagnostics: 1 })
  })
})
