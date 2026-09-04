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
  domainCount: number
  expanded: boolean
  onToggle?: (id: string, project: string) => void
}

export interface DomainNodeData extends GroupNodeData {
  entityKind: "domain"
  architectureId: string
  domain: string
  expanded: boolean
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
  const domainsByArchitecture = new Map<string, typeof visibleGraph.nodes>()
  const filesByDomain = new Map<string, typeof visibleGraph.nodes>()
  for (const entity of visibleGraph.nodes) {
    if (entity.kind === "domain") {
      domainsByArchitecture.set(entity.architectureId, [...(domainsByArchitecture.get(entity.architectureId) ?? []), entity])
    } else if (entity.kind === "file") {
      filesByDomain.set(entity.domainId, [...(filesByDomain.get(entity.domainId) ?? []), entity])
    }
  }
  for (const children of [...domainsByArchitecture.values(), ...filesByDomain.values()]) {
    children.sort((left, right) => left.id.localeCompare(right.id))
  }

  const domainHeight = (domainId: string): number => filesByDomain.has(domainId)
    ? 88 + (filesByDomain.get(domainId)?.length ?? 0) * 78
    : 82
  const architectureHeight = (architectureId: string): number => {
    const domains = domainsByArchitecture.get(architectureId) ?? []
    if (domains.length === 0) return 96
    return 94 + domains.reduce((total, domain) => total + domainHeight(domain.id) + 12, 0) + 12
  }

  const nodes = visibleGraph.nodes.map<FileGraphFlowNode>((entity, index) => {
    const selected = entity.kind === "file" ? selectedPath === entity.path : selectedGroupId === entity.id
    let position = positions[entity.id] ?? { x: entity.rank * 274, y: index * 104 }
    let parentId: string | undefined
    if (entity.kind === "domain") {
      const siblings = domainsByArchitecture.get(entity.architectureId) ?? []
      const siblingIndex = siblings.findIndex((sibling) => sibling.id === entity.id)
      position = {
        x: 18,
        y: 82 + siblings.slice(0, siblingIndex).reduce((total, sibling) => total + domainHeight(sibling.id) + 12, 0),
      }
      parentId = entity.architectureId
    } else if (entity.kind === "file") {
      const siblings = filesByDomain.get(entity.domainId) ?? []
      position = { x: 18, y: 70 + siblings.findIndex((sibling) => sibling.id === entity.id) * 78 }
      parentId = entity.domainId
    }
    const common = {
      id: entity.id,
      position,
      parentId,
      extent: parentId ? "parent" as const : undefined,
      draggable: parentId === undefined,
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
          domainCount: entity.domainCount,
          expanded: entity.expanded,
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
          expanded: entity.expanded,
          fileCount: entity.fileCount,
          internalDependencyCount: entity.internalDependencyCount,
          diagnosticCount: entity.diagnosticCount,
          selected,
          searchMatch: entity.searchMatch,
          rank: entity.rank,
          lane: entity.lane,
        },
        ariaLabel: `${entity.project}, ${ARCHITECTURE_LAYER_LABELS[entity.layer]}, folder ${entity.domain}, ${entity.fileCount} ${entity.fileCount === 1 ? "file" : "files"}${entity.diagnosticCount ? `, ${entity.diagnosticCount} diagnostics` : ""}`,
        style: { width: 224, height: domainHeight(entity.id) },
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
      style: { width: 188 },
    }
  })

  for (const node of nodes) {
    if (node.type === "architecture") node.style = { width: 260, height: architectureHeight(node.id) }
  }

  const selectedIds = new Set<string>()
  if (selectedPath) {
    const selectedFile = visibleGraph.nodes.find((node) => node.kind === "file" && node.path === selectedPath)
    if (selectedFile?.kind === "file") {
      selectedIds.add(selectedFile.id)
      selectedIds.add(selectedFile.domainId)
      selectedIds.add(selectedFile.architectureId)
    } else {
      selectedIds.add(fileNodeId(selectedPath))
    }
  } else if (selectedGroupId) {
    selectedIds.add(selectedGroupId)
    const selectedDomain = visibleGraph.nodes.find((node) => node.kind === "domain" && node.id === selectedGroupId)
    if (selectedDomain?.kind === "domain") selectedIds.add(selectedDomain.architectureId)
  }
  const edges = visibleGraph.edges.map<Edge>((edge) => {
    const connected = selectedIds.has(edge.from) || selectedIds.has(edge.to)
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
