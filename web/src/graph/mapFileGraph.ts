import { MarkerType, type Edge, type Node } from "@xyflow/react"
import type { GraphPosition } from "../types/graphView"
import { ARCHITECTURE_LAYER_LABELS, type ArchitectureLane, type ArchitectureLayer } from "./fileArchitecture"
import { classifyFile, type FileCategory } from "./fileCategory"
import {
  fileEntityId,
  relationshipDetails,
  type VisibleFileGraph,
} from "./hierarchicalFileGraph"

interface GroupNodeData extends Record<string, unknown> {
  id: string
  project: string
  layer: ArchitectureLayer
  layerLabel: string
  fileCount: number
  internalDependencyCount: number
  diagnosticCount: number
  selected: boolean
  searchMatch: boolean
  rank: number
  lane: ArchitectureLane
  onSelect?: (id: string) => void
}

export interface ArchitectureNodeData extends GroupNodeData {
  entityKind: "architecture"
  label: string
  onToggle?: (id: string, project: string) => void
}

export interface DomainNodeData extends GroupNodeData {
  entityKind: "domain"
  architectureId: string
  domain: string
  onToggle?: (id: string, architectureId: string) => void
}

export interface FileNodeData extends Record<string, unknown> {
  entityKind: "file"
  path: string
  fileName: string
  parentPath: string
  category: FileCategory
  project: string
  layer: ArchitectureLayer
  domain: string
  diagnosticCount: number
  selected: boolean
  searchMatch: boolean
  rank: number
  lane: ArchitectureLane
  onSelect?: (path: string) => void
}

export type ArchitectureFlowNode = Node<ArchitectureNodeData, "architecture">
export type DomainFlowNode = Node<DomainNodeData, "domain">
export type FileFlowNode = Node<FileNodeData, "file">
export type FileGraphFlowNode = ArchitectureFlowNode | DomainFlowNode | FileFlowNode

export function fileNodeId(path: string): string {
  return fileEntityId(path)
}

function splitPath(path: string): { fileName: string; parentPath: string } {
  const separator = path.lastIndexOf("/")
  if (separator < 0) return { fileName: path, parentPath: "." }
  return { fileName: path.slice(separator + 1), parentPath: path.slice(0, separator) }
}

export function mapFileGraph(
  visibleGraph: VisibleFileGraph,
  selectedPath: string | null,
  selectedGroupId: string | null,
  search: string,
  positions: Record<string, GraphPosition>,
): { nodes: FileGraphFlowNode[]; edges: Edge[] } {
  const nodes = visibleGraph.nodes.map<FileGraphFlowNode>((entity, index) => {
    const selected = entity.kind === "file" ? selectedPath === entity.path : selectedGroupId === entity.id
    const common = {
      id: entity.id,
      position: positions[entity.id] ?? { x: entity.rank * 274, y: index * 104 },
      draggable: true,
      selectable: true,
      focusable: true,
      connectable: false,
      deletable: false,
      selected,
      className: search.trim() && !entity.searchMatch ? "file-flow-node--dimmed" : "",
    }

    if (entity.kind === "architecture") {
      return {
        ...common,
        type: "architecture",
        data: {
          entityKind: "architecture",
          id: entity.id,
          project: entity.project,
          layer: entity.layer,
          layerLabel: ARCHITECTURE_LAYER_LABELS[entity.layer],
          label: entity.label,
          fileCount: entity.fileCount,
          internalDependencyCount: entity.internalDependencyCount,
          diagnosticCount: entity.diagnosticCount,
          selected,
          searchMatch: entity.searchMatch,
          rank: entity.rank,
          lane: entity.lane,
        },
        ariaLabel: `${entity.project}, ${entity.label}, ${entity.fileCount} ${entity.fileCount === 1 ? "file" : "files"}, ${entity.internalDependencyCount} internal dependencies${entity.diagnosticCount ? `, ${entity.diagnosticCount} diagnostics` : ""}`,
      }
    }

    if (entity.kind === "domain") {
      return {
        ...common,
        type: "domain",
        data: {
          entityKind: "domain",
          id: entity.id,
          project: entity.project,
          layer: entity.layer,
          layerLabel: ARCHITECTURE_LAYER_LABELS[entity.layer],
          architectureId: entity.architectureId,
          domain: entity.domain,
          fileCount: entity.fileCount,
          internalDependencyCount: entity.internalDependencyCount,
          diagnosticCount: entity.diagnosticCount,
          selected,
          searchMatch: entity.searchMatch,
          rank: entity.rank,
          lane: entity.lane,
        },
        ariaLabel: `${entity.project}, ${ARCHITECTURE_LAYER_LABELS[entity.layer]}, domain ${entity.domain}, ${entity.fileCount} ${entity.fileCount === 1 ? "file" : "files"}${entity.diagnosticCount ? `, ${entity.diagnosticCount} diagnostics` : ""}`,
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
        project: entity.project,
        layer: entity.layer,
        domain: entity.domain,
        diagnosticCount: entity.diagnosticCount,
        selected,
        searchMatch: entity.searchMatch,
        rank: entity.rank,
        lane: entity.lane,
      },
      ariaLabel: `${entity.path}${entity.diagnosticCount ? `, ${entity.diagnosticCount} diagnostics` : ""}`,
    }
  })

  const selectedId = selectedPath ? fileNodeId(selectedPath) : selectedGroupId
  const edges = visibleGraph.edges.map<Edge>((edge) => {
    const connected = selectedId !== null && (edge.from === selectedId || edge.to === selectedId)
    const relationship = relationshipDetails(edge.relationship)
    const color = connected ? "#3f444a" : relationship.color
    return {
      id: edge.id,
      source: edge.from,
      target: edge.to,
      type: "smoothstep",
      label: edge.dependencyCount > 1 ? String(edge.dependencyCount) : undefined,
      markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14, color },
      className: `file-edge file-edge--${edge.relationship}${connected ? " file-edge--selected" : ""}`,
      style: {
        stroke: color,
        strokeWidth: Math.min(2.2, 1.2 + Math.log2(edge.dependencyCount) * 0.22),
        opacity: connected ? 1 : 0.76,
      },
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
