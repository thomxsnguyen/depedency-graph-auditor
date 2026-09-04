import type { FileGraphSnapshot } from "../types/fileGraph"
import {
  ARCHITECTURE_LAYER_LABELS,
  classifyFileArchitecture,
  type ArchitectureLane,
  type ArchitectureLayer,
  type FileArchitecture,
} from "./fileArchitecture"
import { fileMatchesSearch } from "./fileGraphSelectors"

export type DependencyHopScope = 1 | 2 | "all"
export type FileRelationship = "main" | "cross-project" | "support" | "test" | "other"

interface VisibleGroupBase {
  id: string
  project: string
  layer: ArchitectureLayer
  fileCount: number
  internalDependencyCount: number
  diagnosticCount: number
  searchMatch: boolean
  rank: number
  lane: ArchitectureLane
}

export interface VisibleArchitectureNode extends VisibleGroupBase {
  kind: "architecture"
  label: string
  domainCount: number
  expanded: boolean
}

export interface VisibleDomainNode extends VisibleGroupBase {
  kind: "domain"
  architectureId: string
  domain: string
  expanded: boolean
}

export interface VisibleFileNode {
  kind: "file"
  id: string
  path: string
  architectureId: string
  domainId: string
  project: string
  layer: ArchitectureLayer
  domain: string
  diagnosticCount: number
  searchMatch: boolean
  rank: number
  lane: ArchitectureLane
}

export type FileGraphEntity = VisibleArchitectureNode | VisibleDomainNode | VisibleFileNode

export interface VisibleFileGraphEdge {
  id: string
  from: string
  to: string
  dependencyCount: number
  relationship: FileRelationship
}

export interface VisibleFileGraph {
  nodes: FileGraphEntity[]
  edges: VisibleFileGraphEdge[]
}

export interface ExpandedArchitectureItem {
  kind: "architecture" | "domain"
  id: string
  label: string
}

export const FILE_RELATIONSHIP_DETAILS: ReadonlyArray<{
  relationship: FileRelationship
  label: string
  color: string
}> = [
  { relationship: "main", label: "Application flow", color: "#6f747b" },
  { relationship: "cross-project", label: "Cross-project", color: "#7a8290" },
  { relationship: "support", label: "Configuration / shared", color: "#8b8278" },
  { relationship: "test", label: "Test dependency", color: "#7d897f" },
  { relationship: "other", label: "Other", color: "#92969c" },
]

export function relationshipDetails(relationship: FileRelationship) {
  return FILE_RELATIONSHIP_DETAILS.find((details) => details.relationship === relationship)!
}

export function fileEntityId(path: string): string {
  return `file:${encodeURIComponent(path)}`
}

export function architectureEntityId(project: string, layer: ArchitectureLayer): string {
  return `architecture:${encodeURIComponent(project)}:${layer}`
}

export function domainEntityId(project: string, layer: ArchitectureLayer, domain: string): string {
  return `domain:${encodeURIComponent(project)}:${layer}:${encodeURIComponent(domain)}`
}

export function architectureForFile(path: string): FileArchitecture {
  return classifyFileArchitecture(path)
}

function diagnosticCounts(snapshot: FileGraphSnapshot): Map<string, number> {
  const counts = new Map<string, number>()
  for (const diagnostic of snapshot.diagnostics) {
    counts.set(diagnostic.path, (counts.get(diagnostic.path) ?? 0) + 1)
  }
  return counts
}

function filesWithinHops(snapshot: FileGraphSnapshot, selectedPath: string, hops: 1 | 2): Set<string> {
  const adjacency = new Map<string, Set<string>>()
  for (const node of snapshot.nodes) adjacency.set(node.path, new Set())
  for (const edge of snapshot.edges) {
    adjacency.get(edge.from)?.add(edge.to)
    adjacency.get(edge.to)?.add(edge.from)
  }

  const visible = new Set([selectedPath])
  let frontier = new Set([selectedPath])
  for (let depth = 0; depth < hops; depth += 1) {
    const next = new Set<string>()
    for (const path of frontier) {
      for (const neighbor of adjacency.get(path) ?? []) {
        if (visible.has(neighbor)) continue
        visible.add(neighbor)
        next.add(neighbor)
      }
    }
    frontier = next
  }
  return visible
}

function classifyRelationship(source: FileArchitecture, target: FileArchitecture): FileRelationship {
  if (source.project !== target.project) return "cross-project"
  if (source.layer === "test" || target.layer === "test") return "test"
  if (["configuration", "shared"].includes(source.layer) || ["configuration", "shared"].includes(target.layer)) {
    return "support"
  }
  const main = new Set<ArchitectureLayer>([
    "entrypoint", "presentation", "transport", "application", "domain", "persistence", "infrastructure",
  ])
  if (main.has(source.layer) && main.has(target.layer)) return "main"
  return "other"
}

function countInternalDependencies(
  snapshot: FileGraphSnapshot,
  filePaths: ReadonlySet<string>,
): number {
  return snapshot.edges.filter((edge) => filePaths.has(edge.from) && filePaths.has(edge.to)).length
}

function groupEntity(
  kind: "architecture" | "domain",
  id: string,
  architecture: FileArchitecture,
  files: readonly string[],
  snapshot: FileGraphSnapshot,
  diagnostics: ReadonlyMap<string, number>,
  search: string,
): VisibleArchitectureNode | VisibleDomainNode {
  const fileSet = new Set(files)
  const common = {
    id,
    project: architecture.project,
    layer: architecture.layer,
    fileCount: files.length,
    internalDependencyCount: countInternalDependencies(snapshot, fileSet),
    diagnosticCount: files.reduce((total, path) => total + (diagnostics.get(path) ?? 0), 0),
    searchMatch: files.some((path) => fileMatchesSearch(path, search)),
    rank: architecture.rank,
    lane: architecture.lane,
  }
  if (kind === "architecture") {
    return {
      ...common,
      kind,
      label: ARCHITECTURE_LAYER_LABELS[architecture.layer],
      domainCount: 0,
      expanded: false,
    }
  }
  return {
    ...common,
    kind,
    architectureId: architectureEntityId(architecture.project, architecture.layer),
    domain: architecture.domain,
    expanded: false,
  }
}

export function buildHierarchicalFileGraph(
  snapshot: FileGraphSnapshot,
  expandedArchitectureIds: ReadonlySet<string>,
  expandedDomainIds: ReadonlySet<string>,
  selectedPath: string | null,
  hopScope: DependencyHopScope,
  search = "",
): VisibleFileGraph {
  const architectureByFile = new Map(snapshot.nodes.map((node) => [node.path, architectureForFile(node.path)]))
  const filesByArchitecture = new Map<string, string[]>()
  const filesByDomain = new Map<string, string[]>()
  for (const node of snapshot.nodes) {
    const architecture = architectureByFile.get(node.path)!
    const architectureId = architectureEntityId(architecture.project, architecture.layer)
    const domainId = domainEntityId(architecture.project, architecture.layer, architecture.domain)
    filesByArchitecture.set(architectureId, [...(filesByArchitecture.get(architectureId) ?? []), node.path])
    filesByDomain.set(domainId, [...(filesByDomain.get(domainId) ?? []), node.path])
  }
  for (const files of [...filesByArchitecture.values(), ...filesByDomain.values()]) files.sort()

  const diagnostics = diagnosticCounts(snapshot)
  const allNodes: FileGraphEntity[] = []
  const representedFiles = new Map<string, readonly string[]>()

  for (const [architectureId, architectureFiles] of [...filesByArchitecture].sort(([left], [right]) => left.localeCompare(right))) {
    const architecture = architectureByFile.get(architectureFiles[0])!
    const domainEntries = [...filesByDomain]
      .filter(([, files]) => architectureByFile.get(files[0])?.project === architecture.project
        && architectureByFile.get(files[0])?.layer === architecture.layer)
      .sort(([left], [right]) => left.localeCompare(right))

    allNodes.push({
      ...groupEntity("architecture", architectureId, architecture, architectureFiles, snapshot, diagnostics, search),
      kind: "architecture",
      label: ARCHITECTURE_LAYER_LABELS[architecture.layer],
      domainCount: domainEntries.length,
      expanded: expandedArchitectureIds.has(architectureId),
    })
    representedFiles.set(architectureId, architectureFiles)
    if (!expandedArchitectureIds.has(architectureId)) continue

    for (const [domainId, domainFiles] of domainEntries) {
      const domainArchitecture = architectureByFile.get(domainFiles[0])!
      allNodes.push({
        ...groupEntity("domain", domainId, domainArchitecture, domainFiles, snapshot, diagnostics, search),
        kind: "domain",
        architectureId,
        domain: domainArchitecture.domain,
        expanded: expandedDomainIds.has(domainId),
      })
      representedFiles.set(domainId, domainFiles)
      if (!expandedDomainIds.has(domainId)) continue
      for (const path of domainFiles) {
        const fileArchitecture = architectureByFile.get(path)!
        const id = fileEntityId(path)
        allNodes.push({
          kind: "file",
          id,
          path,
          architectureId,
          domainId,
          project: fileArchitecture.project,
          layer: fileArchitecture.layer,
          domain: fileArchitecture.domain,
          diagnosticCount: diagnostics.get(path) ?? 0,
          searchMatch: fileMatchesSearch(path, search),
          rank: fileArchitecture.rank,
          lane: fileArchitecture.lane,
        })
        representedFiles.set(id, [path])
      }
    }
  }

  const kindOrder = { architecture: 0, domain: 1, file: 2 } as const
  allNodes.sort((left, right) => kindOrder[left.kind] - kindOrder[right.kind] || left.id.localeCompare(right.id))

  const detailedEndpoint = (path: string): string | null => {
    const architecture = architectureByFile.get(path)
    if (!architecture) return null
    const architectureId = architectureEntityId(architecture.project, architecture.layer)
    if (!expandedArchitectureIds.has(architectureId)) return architectureId
    const domainId = domainEntityId(architecture.project, architecture.layer, architecture.domain)
    return expandedDomainIds.has(domainId) ? fileEntityId(path) : domainId
  }

  const aggregatedEdges = new Map<string, VisibleFileGraphEdge>()
  for (const edge of snapshot.edges) {
    const sourceArchitecture = architectureByFile.get(edge.from)
    const targetArchitecture = architectureByFile.get(edge.to)
    if (!sourceArchitecture || !targetArchitecture) continue
    const sourceArchitectureId = architectureEntityId(sourceArchitecture.project, sourceArchitecture.layer)
    const targetArchitectureId = architectureEntityId(targetArchitecture.project, targetArchitecture.layer)
    const crossesArchitecture = sourceArchitectureId !== targetArchitectureId
    const from = crossesArchitecture ? sourceArchitectureId : detailedEndpoint(edge.from)
    const to = crossesArchitecture ? targetArchitectureId : detailedEndpoint(edge.to)
    if (!from || !to || from === to) continue
    const key = `${from}\u0000${to}`
    const current = aggregatedEdges.get(key)
    if (current) {
      current.dependencyCount += 1
    } else {
      aggregatedEdges.set(key, {
        id: `file-edge:${encodeURIComponent(from)}=>${encodeURIComponent(to)}`,
        from,
        to,
        dependencyCount: 1,
        relationship: classifyRelationship(sourceArchitecture, targetArchitecture),
      })
    }
  }

  const focusedFiles = selectedPath && hopScope !== "all"
    ? filesWithinHops(snapshot, selectedPath, hopScope)
    : null
  const searchActive = search.trim().length > 0
  const visibleNodeIds = new Set(allNodes.flatMap((node) => {
    if (!focusedFiles) return [node.id]
    const inFocus = (representedFiles.get(node.id) ?? []).some((path) => focusedFiles.has(path))
    return inFocus || (searchActive && node.searchMatch) ? [node.id] : []
  }))
  for (const node of allNodes) {
    if (!visibleNodeIds.has(node.id)) continue
    if (node.kind === "domain") visibleNodeIds.add(node.architectureId)
    if (node.kind === "file") {
      visibleNodeIds.add(node.domainId)
      visibleNodeIds.add(node.architectureId)
    }
  }

  return {
    nodes: allNodes.filter((node) => visibleNodeIds.has(node.id)),
    edges: [...aggregatedEdges.values()]
      .filter((edge) => visibleNodeIds.has(edge.from) && visibleNodeIds.has(edge.to))
      .sort((left, right) => left.id.localeCompare(right.id)),
  }
}

export function expandedArchitectureItems(
  snapshot: FileGraphSnapshot,
  expandedArchitectureIds: ReadonlySet<string>,
  expandedDomainIds: ReadonlySet<string>,
): ExpandedArchitectureItem[] {
  const items = new Map<string, ExpandedArchitectureItem>()
  for (const node of snapshot.nodes) {
    const architecture = architectureForFile(node.path)
    const architectureId = architectureEntityId(architecture.project, architecture.layer)
    if (expandedArchitectureIds.has(architectureId)) {
      items.set(architectureId, {
        kind: "architecture",
        id: architectureId,
        label: `${architecture.project} / ${ARCHITECTURE_LAYER_LABELS[architecture.layer]}`,
      })
    }
    const domainId = domainEntityId(architecture.project, architecture.layer, architecture.domain)
    if (expandedDomainIds.has(domainId)) {
      items.set(domainId, {
        kind: "domain",
        id: domainId,
        label: `${architecture.project} / ${ARCHITECTURE_LAYER_LABELS[architecture.layer]} / ${architecture.domain}`,
      })
    }
  }
  return [...items.values()].sort((left, right) => left.label.localeCompare(right.label))
}
