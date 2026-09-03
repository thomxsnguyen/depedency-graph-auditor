import fixture from "./sample-file-graph.json"
import type { FileGraphDataSource } from "./FileGraphDataSource"
import type { FileGraphDiagnostic, FileGraphEdge, FileGraphNode, FileGraphSnapshot } from "../types/fileGraph"

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0
}

export function parseFileGraph(value: unknown): FileGraphSnapshot {
  if (!isRecord(value) || value.schemaVersion !== 1 || !isNonEmptyString(value.root)) {
    throw new Error("Unsupported file graph schema")
  }
  if (!Array.isArray(value.nodes) || !Array.isArray(value.edges) || !Array.isArray(value.diagnostics)) {
    throw new Error("Invalid file graph collections")
  }

  const nodes: FileGraphNode[] = value.nodes.map((node) => {
    if (!isRecord(node) || !isNonEmptyString(node.path)) throw new Error("Invalid file graph node")
    return { path: node.path }
  })
  const nodePaths = new Set(nodes.map((node) => node.path))
  if (nodePaths.size !== nodes.length) throw new Error("Duplicate file graph node")

  const edges: FileGraphEdge[] = value.edges.map((edge) => {
    if (!isRecord(edge) || !isNonEmptyString(edge.from) || !isNonEmptyString(edge.to)) {
      throw new Error("Invalid file graph edge")
    }
    if (!nodePaths.has(edge.from) || !nodePaths.has(edge.to)) {
      throw new Error("File graph edge references an unknown node")
    }
    return { from: edge.from, to: edge.to }
  })

  const diagnostics: FileGraphDiagnostic[] = value.diagnostics.map((diagnostic) => {
    if (
      !isRecord(diagnostic)
      || !isNonEmptyString(diagnostic.path)
      || !isNonEmptyString(diagnostic.message)
      || (diagnostic.import !== undefined && typeof diagnostic.import !== "string")
    ) {
      throw new Error("Invalid file graph diagnostic")
    }
    return {
      path: diagnostic.path,
      message: diagnostic.message,
      ...(diagnostic.import === undefined ? {} : { import: diagnostic.import }),
    }
  })

  return { schemaVersion: 1, root: value.root, nodes, edges, diagnostics }
}

export class FixtureFileGraphDataSource implements FileGraphDataSource {
  load(): Promise<FileGraphSnapshot> {
    return Promise.resolve().then(() => parseFileGraph(fixture))
  }
}
