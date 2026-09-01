import { MarkerType, type Edge, type Node } from "@xyflow/react"
import type { AuditPackage, AuditSnapshot } from "../types/audit"
import type { GraphPosition } from "../types/graphView"
import { findPath, ROOT_NODE_ID } from "./graphSelectors"

export interface AuditNodeData extends Record<string, unknown> {
  label: string
  packageRow: AuditPackage | null
  root: boolean
  selectedPath: boolean
  collapsed: boolean
  annotated: boolean
  onSelect?: (nodeId: string) => void
  onCollapse?: (nodeId: string) => void
}

export type AuditGraphNode = Node<AuditNodeData, "auditPackage">

export function mapAuditGraph(
  snapshot: AuditSnapshot,
  packages: readonly AuditPackage[],
  visibleEdges: AuditSnapshot["edges"],
  selectedNodeId: string | null,
  collapsedNodeIds: readonly string[],
  annotations: Record<string, string>,
  pinnedPositions: Record<string, GraphPosition>,
): { nodes: AuditGraphNode[]; edges: Edge[]; selectedPath: string[] } {
  const selectedPath = selectedNodeId ? findPath(selectedNodeId, snapshot.edges) : []
  const selectedIds = new Set(selectedPath)
  const selectedEdgeIds = new Set<string>()

  for (let index = 0; index < selectedPath.length - 1; index += 1) {
    selectedEdgeIds.add(`${selectedPath[index]}::${selectedPath[index + 1]}`)
  }

  const rows: Array<AuditPackage | null> = [null, ...packages]
  const nodes = rows.map<AuditGraphNode>((packageRow, index) => {
    const id = packageRow?.id ?? ROOT_NODE_ID
    return {
      id,
      type: "auditPackage",
      position: pinnedPositions[id] ?? { x: index * 220, y: 80 },
      data: {
        label: packageRow ? `${packageRow.name}@${packageRow.version}` : snapshot.root,
        packageRow,
        root: !packageRow,
        selectedPath: selectedIds.has(id),
        collapsed: collapsedNodeIds.includes(id),
        annotated: Boolean(annotations[id]?.trim()),
      },
      draggable: true,
      selectable: true,
      focusable: true,
      connectable: false,
      deletable: false,
      ariaLabel: packageRow
        ? `${packageRow.name} version ${packageRow.version}, ${packageRow.status.replaceAll("_", " ")}`
        : `${snapshot.root}, audit root`,
    }
  })

  const edges = visibleEdges.map<Edge>((edge) => {
    const selected = selectedEdgeIds.has(`${edge.from}::${edge.to}`)
    return {
      id: edge.id,
      source: edge.from,
      target: edge.to,
      type: "smoothstep",
      markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 },
      className: selected ? "dependency-edge dependency-edge--selected" : "dependency-edge",
      animated: false,
      focusable: false,
      deletable: false,
    }
  })

  return { nodes, edges, selectedPath }
}
