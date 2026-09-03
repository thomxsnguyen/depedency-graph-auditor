export interface FileGraphNode {
  path: string
}

export interface FileGraphEdge {
  from: string
  to: string
}

export interface FileGraphDiagnostic {
  path: string
  import?: string
  message: string
}

export interface FileGraphSnapshot {
  schemaVersion: 1
  root: string
  nodes: FileGraphNode[]
  edges: FileGraphEdge[]
  diagnostics: FileGraphDiagnostic[]
}

export type GraphMode = "dependencies" | "files"
