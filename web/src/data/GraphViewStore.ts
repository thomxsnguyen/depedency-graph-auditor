import type { GraphView } from "../types/graphView"

export interface GraphViewStore {
  load(auditId: string): Promise<GraphView | null>
  save(auditId: string, view: GraphView): Promise<void>
}
