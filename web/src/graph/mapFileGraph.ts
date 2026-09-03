import { MarkerType, type Edge, type Node } from "@xyflow/react"
import type { FileGraphSnapshot } from "../types/fileGraph"
import type { GraphPosition } from "../types/graphView"
import { fileMatchesSearch } from "./fileGraphSelectors"

export interface FileNodeData extends Record<string, unknown> {
  path: string
  fileName: string
  parentPath: string
  diagnosticCount: number
  selected: boolean
  searchMatch: boolean
  onSelect?: (path: string) => void
}

export type FileGraphFlowNode = Node<FileNodeData, "file">

export function fileNodeId(path: string): string {
  return `file:${encodeURIComponent(path)}`
}

export function fileEdgeId(from: string, to: string): string {
  return `file-edge:${encodeURIComponent(from)}=>${encodeURIComponent(to)}`
}

function splitPath(path: string): { fileName: string; parentPath: string } {
  const separator = path.lastIndexOf("/")
  if (separator < 0) return { fileName: path, parentPath: "." }
  return { fileName: path.slice(separator + 1), parentPath: path.slice(0, separator) }
}

export function mapFileGraph(
  snapshot: FileGraphSnapshot,
  selectedPath: string | null,
  search: string,
  positions: Record<string, GraphPosition>,
): { nodes: FileGraphFlowNode[]; edges: Edge[] } {
  const diagnosticCounts = new Map<string, number>()
  for (const diagnostic of snapshot.diagnostics) {
    diagnosticCounts.set(diagnostic.path, (diagnosticCounts.get(diagnostic.path) ?? 0) + 1)
  }

  const nodes = snapshot.nodes.map<FileGraphFlowNode>((node, index) => {
    const labels = splitPath(node.path)
    const id = fileNodeId(node.path)
    const searchMatch = fileMatchesSearch(node.path, search)
    return {
      id,
      type: "file",
      position: positions[node.path] ?? { x: (index % 4) * 240, y: Math.floor(index / 4) * 110 },
      data: {
        path: node.path,
        ...labels,
        diagnosticCount: diagnosticCounts.get(node.path) ?? 0,
        selected: selectedPath === node.path,
        searchMatch,
      },
      draggable: true,
      selectable: true,
      focusable: true,
      connectable: false,
      deletable: false,
      selected: selectedPath === node.path,
      className: search.trim() && !searchMatch ? "file-flow-node--dimmed" : "",
      ariaLabel: `${node.path}${diagnosticCounts.has(node.path) ? `, ${diagnosticCounts.get(node.path)} diagnostics` : ""}`,
    }
  })

  const edges = snapshot.edges.map<Edge>((edge) => {
    const connected = selectedPath !== null && (edge.from === selectedPath || edge.to === selectedPath)
    return {
      id: fileEdgeId(edge.from, edge.to),
      source: fileNodeId(edge.from),
      target: fileNodeId(edge.to),
      type: "smoothstep",
      markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 },
      className: connected ? "file-edge file-edge--selected" : "file-edge",
      animated: false,
      focusable: false,
      deletable: false,
    }
  })

  return { nodes, edges }
}
