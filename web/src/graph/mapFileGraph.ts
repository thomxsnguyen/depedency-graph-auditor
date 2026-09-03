import { MarkerType, type Edge, type Node } from "@xyflow/react"
import type { GraphPosition } from "../types/graphView"
import { classifyFile, type FileCategory } from "./fileCategory"
import type { VisibleFileGraph } from "./hierarchicalFileGraph"

export interface FileNodeData extends Record<string, unknown> {
  entityKind: "file"
  path: string
  fileName: string
  parentPath: string
  category: FileCategory
  diagnosticCount: number
  selected: boolean
  searchMatch: boolean
  onSelect?: (path: string) => void
}

export interface ModuleNodeData extends Record<string, unknown> {
  entityKind: "module"
  path: string
  fileCount: number
  internalDependencyCount: number
  diagnosticCount: number
  selected: boolean
  searchMatch: boolean
  onSelect?: (path: string) => void
  onToggle?: (path: string) => void
}

export type FileFlowNode = Node<FileNodeData, "file">
export type ModuleFlowNode = Node<ModuleNodeData, "module">
export type FileGraphFlowNode = FileFlowNode | ModuleFlowNode

export function fileNodeId(path: string): string {
  return `file:${encodeURIComponent(path)}`
}

export function moduleNodeId(path: string): string {
  return `module:${encodeURIComponent(path)}`
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
  visibleGraph: VisibleFileGraph,
  selectedPath: string | null,
  selectedModulePath: string | null,
  search: string,
  positions: Record<string, GraphPosition>,
): { nodes: FileGraphFlowNode[]; edges: Edge[] } {
  const nodes = visibleGraph.nodes.map<FileGraphFlowNode>((entity, index) => {
    const selected = entity.kind === "file"
      ? selectedPath === entity.path
      : selectedModulePath === entity.path
    const common = {
      id: entity.id,
      position: positions[entity.id] ?? { x: (index % 4) * 240, y: Math.floor(index / 4) * 110 },
      draggable: true,
      selectable: true,
      focusable: true,
      connectable: false,
      deletable: false,
      selected,
      className: search.trim() && !entity.searchMatch ? "file-flow-node--dimmed" : "",
    }

    if (entity.kind === "module") {
      return {
        ...common,
        type: "module",
        data: {
          entityKind: "module",
          path: entity.path,
          fileCount: entity.fileCount,
          internalDependencyCount: entity.internalDependencyCount,
          diagnosticCount: entity.diagnosticCount,
          selected,
          searchMatch: entity.searchMatch,
        },
        ariaLabel: `Module ${entity.path}, ${entity.fileCount} ${entity.fileCount === 1 ? "file" : "files"}, ${entity.internalDependencyCount} internal dependencies${entity.diagnosticCount ? `, ${entity.diagnosticCount} diagnostics` : ""}`,
      }
    }

    return {
      ...common,
      type: "file",
      data: {
        entityKind: "file",
        path: entity.path,
        ...splitPath(entity.path),
        category: classifyFile(entity.path),
        diagnosticCount: entity.diagnosticCount,
        selected,
        searchMatch: entity.searchMatch,
      },
      ariaLabel: `${entity.path}${entity.diagnosticCount ? `, ${entity.diagnosticCount} diagnostics` : ""}`,
    }
  })

  const selectedId = selectedPath ? fileNodeId(selectedPath) : selectedModulePath ? moduleNodeId(selectedModulePath) : null
  const edges = visibleGraph.edges.map<Edge>((edge) => {
    const connected = selectedId !== null && (edge.from === selectedId || edge.to === selectedId)
    return {
      id: edge.id,
      source: edge.from,
      target: edge.to,
      type: "smoothstep",
      label: edge.dependencyCount > 1 ? String(edge.dependencyCount) : undefined,
      markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 },
      className: connected ? "file-edge file-edge--selected" : "file-edge",
      style: { strokeWidth: Math.min(2.2, 1.2 + Math.log2(edge.dependencyCount) * 0.22) },
      labelStyle: { fill: "#6e6e73", fontSize: 10, fontWeight: 600 },
      labelBgStyle: { fill: "#f5f5f7", fillOpacity: 0.92 },
      labelBgPadding: [4, 2],
      animated: false,
      focusable: false,
      deletable: false,
    }
  })

  return { nodes, edges }
}
