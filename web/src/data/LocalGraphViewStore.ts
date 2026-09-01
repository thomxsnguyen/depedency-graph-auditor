import type { GraphViewStore } from "./GraphViewStore"
import type { GraphView } from "../types/graphView"

// Version the demo presentation state so incompatible viewport/layout changes
// cannot strand the graph outside the visible canvas after an upgrade.
const keyForAudit = (auditId: string) => `dependency-audit-view:v2:${auditId}`

export class LocalGraphViewStore implements GraphViewStore {
  async load(auditId: string): Promise<GraphView | null> {
    const raw = window.localStorage.getItem(keyForAudit(auditId))
    if (!raw) return null

    const parsed = JSON.parse(raw) as GraphView
    return parsed
  }

  async save(auditId: string, view: GraphView): Promise<void> {
    window.localStorage.setItem(keyForAudit(auditId), JSON.stringify(view))
  }
}
