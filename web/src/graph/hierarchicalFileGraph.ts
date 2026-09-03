import type { FileGraphSnapshot } from "../types/fileGraph"
import { fileMatchesSearch } from "./fileGraphSelectors"

export type DependencyHopScope = 1 | 2 | "all"

export interface VisibleModuleNode {
  kind: "module"
  id: string
  path: string
  fileCount: number
  internalDependencyCount: number
  diagnosticCount: number
  searchMatch: boolean
}

export interface VisibleFileNode {
  kind: "file"
  id: string
  path: string
  modulePath: string
  diagnosticCount: number
  searchMatch: boolean
}

export type FileGraphEntity = VisibleModuleNode | VisibleFileNode

export interface VisibleFileGraphEdge {
  id: string
  from: string
  to: string
  dependencyCount: number
}

export interface VisibleFileGraph {
  nodes: FileGraphEntity[]
  edges: VisibleFileGraphEdge[]
}

export function fileEntityId(path: string): string {
  return `file:${encodeURIComponent(path)}`
}

export function moduleEntityId(path: string): string {
  return `module:${encodeURIComponent(path)}`
}

export function modulePathForFile(path: string): string {
  const parts = path.replaceAll("\\", "/").split("/").filter(Boolean)
  const directories = parts.slice(0, -1)
  if (directories.length === 0) return "."
  return directories.slice(0, 2).join("/")
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

export function buildHierarchicalFileGraph(
  snapshot: FileGraphSnapshot,
  expandedModulePaths: ReadonlySet<string>,
  selectedPath: string | null,
  hopScope: DependencyHopScope,
  search = "",
): VisibleFileGraph {
  const moduleForFile = new Map(snapshot.nodes.map((node) => [node.path, modulePathForFile(node.path)]))
  const filesByModule = new Map<string, string[]>()
  for (const node of snapshot.nodes) {
    const modulePath = moduleForFile.get(node.path)!
    const files = filesByModule.get(modulePath) ?? []
    files.push(node.path)
    filesByModule.set(modulePath, files)
  }
  for (const files of filesByModule.values()) files.sort()

  const diagnostics = diagnosticCounts(snapshot)
  const internalDependencies = new Map<string, number>()
  for (const edge of snapshot.edges) {
    const sourceModule = moduleForFile.get(edge.from)
    if (sourceModule && sourceModule === moduleForFile.get(edge.to)) {
      internalDependencies.set(sourceModule, (internalDependencies.get(sourceModule) ?? 0) + 1)
    }
  }

  const allNodes: FileGraphEntity[] = []
  for (const [modulePath, files] of [...filesByModule].sort(([left], [right]) => left.localeCompare(right))) {
    if (expandedModulePaths.has(modulePath)) {
      for (const path of files) {
        allNodes.push({
          kind: "file",
          id: fileEntityId(path),
          path,
          modulePath,
          diagnosticCount: diagnostics.get(path) ?? 0,
          searchMatch: fileMatchesSearch(path, search),
        })
      }
      continue
    }
    allNodes.push({
      kind: "module",
      id: moduleEntityId(modulePath),
      path: modulePath,
      fileCount: files.length,
      internalDependencyCount: internalDependencies.get(modulePath) ?? 0,
      diagnosticCount: files.reduce((total, path) => total + (diagnostics.get(path) ?? 0), 0),
      searchMatch: files.some((path) => fileMatchesSearch(path, search)),
    })
  }

  const visibleEndpoint = (path: string): string | null => {
    const modulePath = moduleForFile.get(path)
    if (!modulePath) return null
    return expandedModulePaths.has(modulePath) ? fileEntityId(path) : moduleEntityId(modulePath)
  }

  const aggregatedEdges = new Map<string, VisibleFileGraphEdge>()
  for (const edge of snapshot.edges) {
    const from = visibleEndpoint(edge.from)
    const to = visibleEndpoint(edge.to)
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
      })
    }
  }

  const focusedFiles = selectedPath && hopScope !== "all"
    ? filesWithinHops(snapshot, selectedPath, hopScope)
    : null
  const searchActive = search.trim().length > 0
  const visibleNodeIds = new Set(allNodes.flatMap((node) => {
    if (!focusedFiles) return [node.id]
    const representedFiles = node.kind === "file" ? [node.path] : filesByModule.get(node.path) ?? []
    const inFocus = representedFiles.some((path) => focusedFiles.has(path))
    return inFocus || (searchActive && node.searchMatch) ? [node.id] : []
  }))

  return {
    nodes: allNodes.filter((node) => visibleNodeIds.has(node.id)),
    edges: [...aggregatedEdges.values()]
      .filter((edge) => visibleNodeIds.has(edge.from) && visibleNodeIds.has(edge.to))
      .sort((left, right) => left.id.localeCompare(right.id)),
  }
}
