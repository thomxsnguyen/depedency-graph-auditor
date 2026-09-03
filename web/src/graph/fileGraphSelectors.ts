import type { FileGraphSnapshot } from "../types/fileGraph"

export function selectedFile(snapshot: FileGraphSnapshot, path: string | null) {
  return path === null ? null : snapshot.nodes.find((node) => node.path === path) ?? null
}

export function incomingFiles(snapshot: FileGraphSnapshot, path: string): string[] {
  return [...new Set(snapshot.edges.filter((edge) => edge.to === path).map((edge) => edge.from))].sort()
}

export function outgoingFiles(snapshot: FileGraphSnapshot, path: string): string[] {
  return [...new Set(snapshot.edges.filter((edge) => edge.from === path).map((edge) => edge.to))].sort()
}

export function diagnosticsForFile(snapshot: FileGraphSnapshot, path: string) {
  return snapshot.diagnostics.filter((diagnostic) => diagnostic.path === path)
}

export function fileMatchesSearch(path: string, search: string): boolean {
  const query = search.trim().toLocaleLowerCase()
  return query.length === 0 || path.toLocaleLowerCase().includes(query)
}

export function fileGraphCounts(snapshot: FileGraphSnapshot) {
  return {
    files: snapshot.nodes.length,
    imports: snapshot.edges.length,
    diagnostics: snapshot.diagnostics.length,
  }
}
