import completeAudit from "./complete-audit.json"
import type { AuditDataSource } from "./AuditDataSource"
import { countStatuses, type AuditEvent, type AuditSnapshot } from "../types/audit"

function cloneSnapshot(): AuditSnapshot {
  const fixture = structuredClone(completeAudit) as Omit<AuditSnapshot, "counts">
  return {
    ...fixture,
    counts: countStatuses(fixture.packages),
  }
}

export class FixtureAuditDataSource implements AuditDataSource {
  async loadAudit(_auditId: string): Promise<AuditSnapshot> {
    void _auditId
    return cloneSnapshot()
  }

  subscribe(_auditId: string, onEvent: (event: AuditEvent) => void): () => void {
    void _auditId
    void onEvent
    return () => undefined
  }
}
