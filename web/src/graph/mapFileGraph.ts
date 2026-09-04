import { MarkerType, type Edge, type Node } from "@xyflow/react"
import type { GraphPosition } from "../types/graphView"
import { ARCHITECTURE_LAYER_LABELS, type ArchitectureLane, type ArchitectureLayer } from "./fileArchitecture"
import { classifyFile, type FileCategory } from "./fileCategory"
import {
  fileEntityId,
  relationshipDetails,
  type FileRelationship,
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
export type ConnectionDisplayMode = "focused" | "all"

export function relationshipStrokePattern(relationship: FileRelationship): string | undefined {
  return {
    main: undefined,
    "cross-project": "10 6",
    support: "2 5",
    test: "5 4",
    other: "3 6",
  }[relationship]
}

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
  connectionMode: ConnectionDisplayMode = "focused",
  hoveredNodeId: string | null = null,
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

  const fileColumnCount = (domainId: string): number => (filesByDomain.get(domainId)?.length ?? 0) > 6 ? 2 : 1
  const domainHeight = (domainId: string): number => filesByDomain.has(domainId)
    ? 88 + Math.ceil((filesByDomain.get(domainId)?.length ?? 0) / fileColumnCount(domainId)) * 78
    : 82
  const architectureWidth = (architectureId: string): number => {
    const hasWideFolder = (domainsByArchitecture.get(architectureId) ?? [])
      .some((domain) => fileColumnCount(domain.id) === 2)
    return hasWideFolder ? 520 : 300
  }
  const architectureHeight = (architectureId: string): number => {
    const domains = domainsByArchitecture.get(architectureId) ?? []
    if (domains.length === 0) return 96
    return 94 + domains.reduce((total, domain) => total + domainHeight(domain.id) + 12, 0) + 12
  }

  const selectedEntityId = selectedPath ? fileNodeId(selectedPath) : selectedGroupId
  const activeEntityId = hoveredNodeId ?? selectedEntityId
  const activeEntity = visibleGraph.nodes.find((entity) => entity.id === activeEntityId)
  const activeIds = new Set<string>()
  if (activeEntity) {
    activeIds.add(activeEntity.id)
    if (activeEntity.kind === "domain") activeIds.add(activeEntity.architectureId)
    if (activeEntity.kind === "file") {
      activeIds.add(activeEntity.domainId)
      activeIds.add(activeEntity.architectureId)
    }
  }
  const focusedEdges = new Set(visibleGraph.edges
    .filter((edge) => activeIds.has(edge.from) || activeIds.has(edge.to))
    .map((edge) => edge.id))
  const relatedIds = new Set(activeIds)
  for (const edge of visibleGraph.edges) {
    if (!focusedEdges.has(edge.id)) continue
    relatedIds.add(edge.from)
    relatedIds.add(edge.to)
  }
  const isRelatedNode = (entity: VisibleFileGraph["nodes"][number]): boolean => {
    if (relatedIds.has(entity.id)) return true
    if (entity.kind === "domain") return relatedIds.has(entity.architectureId)
    if (entity.kind === "file") return relatedIds.has(entity.domainId) || relatedIds.has(entity.architectureId)
    return false
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
      const siblingIndex = siblings.findIndex((sibling) => sibling.id === entity.id)
      const columns = fileColumnCount(entity.domainId)
      position = {
        x: 18 + (siblingIndex % columns) * 238,
        y: 70 + Math.floor(siblingIndex / columns) * 78,
      }
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
      className: [
        search.trim() && !entity.searchMatch ? "file-flow-node--dimmed" : "",
        connectionMode === "focused" && activeIds.size > 0 && !isRelatedNode(entity)
          ? "file-flow-node--connection-dimmed"
          : "",
      ].filter(Boolean).join(" "),
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
        style: { width: architectureWidth(entity.architectureId) - 36, height: domainHeight(entity.id) },
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
      style: { width: 220 },
    }
  })

  for (const node of nodes) {
    if (node.type === "architecture") {
      node.style = { width: architectureWidth(node.id), height: architectureHeight(node.id) }
    }
  }

  const displayedEdges = connectionMode === "all"
    ? visibleGraph.edges
    : visibleGraph.edges.filter((edge) => focusedEdges.has(edge.id))

  const edges = displayedEdges.map<Edge>((edge) => {
    const connected = focusedEdges.has(edge.id)
    const relationship = relationshipDetails(edge.relationship)
    const color = connected ? "#3f444a" : relationship.color
    const opacity = connected ? 0.96 : 0.42
    return {
      id: edge.id,
      source: edge.from,
      target: edge.to,
      type: "smoothstep",
      sourceHandle: `source-${edge.relationship}`,
      targetHandle: `target-${edge.relationship}`,
      label: connected && edge.dependencyCount > 1 ? `${edge.dependencyCount} imports` : undefined,
      markerEnd: { type: MarkerType.ArrowClosed, width: 11, height: 11, color },
      className: `file-edge file-edge--${edge.relationship}${connected ? " file-edge--selected" : ""}`,
      style: {
        stroke: color,
        strokeWidth: connected ? 1.8 : Math.min(1.45, 1 + Math.log2(edge.dependencyCount) * 0.14),
        strokeDasharray: relationshipStrokePattern(edge.relationship),
        opacity,
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
