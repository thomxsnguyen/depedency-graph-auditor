import type { AuditPackage, AuditSnapshot, DependencyEdge } from "../types/audit"
import type { GraphFilters } from "../types/graphView"

export const ROOT_NODE_ID = "root"

export function directDependencyIds(edges: readonly DependencyEdge[]): Set<string> {
  return new Set(edges.filter((edge) => edge.from === ROOT_NODE_ID).map((edge) => edge.to))
}

export function findPath(
  selectedId: string,
  edges: readonly DependencyEdge[],
): string[] {
  if (selectedId === ROOT_NODE_ID) return [ROOT_NODE_ID]

  const children = new Map<string, string[]>()
  for (const edge of edges) {
    const next = children.get(edge.from) ?? []
    next.push(edge.to)
    children.set(edge.from, next.sort())
  }

  const queue: string[][] = [[ROOT_NODE_ID]]
  const visited = new Set([ROOT_NODE_ID])

  while (queue.length > 0) {
    const path = queue.shift()!
    const last = path[path.length - 1]
    for (const child of children.get(last) ?? []) {
      const nextPath = [...path, child]
      if (child === selectedId) return nextPath
      if (!visited.has(child)) {
        visited.add(child)
        queue.push(nextPath)
      }
    }
  }

  return [selectedId]
}

function descendantsOf(nodeId: string, edges: readonly DependencyEdge[]): Set<string> {
  const hidden = new Set<string>()
  const queue = [nodeId]
  const visited = new Set([nodeId])

  while (queue.length > 0) {
    const current = queue.shift()!
    for (const edge of edges) {
      if (edge.from !== current || visited.has(edge.to)) continue
      visited.add(edge.to)
      hidden.add(edge.to)
      queue.push(edge.to)
    }
  }

  return hidden
}

export function visibleGraph(
  snapshot: AuditSnapshot,
  filters: GraphFilters,
  collapsedNodeIds: readonly string[],
): { packages: AuditPackage[]; edges: DependencyEdge[] } {
  const direct = directDependencyIds(snapshot.edges)
  const normalizedSearch = filters.search.trim().toLocaleLowerCase()
  const hidden = new Set<string>()

  for (const nodeId of collapsedNodeIds) {
    for (const descendant of descendantsOf(nodeId, snapshot.edges)) hidden.add(descendant)
  }

  const packages = snapshot.packages.filter((packageRow) => {
    if (hidden.has(packageRow.id)) return false
    if (filters.violationsOnly && packageRow.verdict !== "policy_violation") return false
    if (filters.directOnly && !direct.has(packageRow.id)) return false
    if (!normalizedSearch) return true
    return `${packageRow.name} ${packageRow.version}`.toLocaleLowerCase().includes(normalizedSearch)
  })

  const visibleIds = new Set([ROOT_NODE_ID, ...packages.map((packageRow) => packageRow.id)])
  const edges = snapshot.edges.filter(
    (edge) => visibleIds.has(edge.from) && visibleIds.has(edge.to),
  )

  return { packages, edges }
}
