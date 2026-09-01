import type { AuditEvent, AuditSnapshot } from "../types/audit"

export interface AuditDataSource {
  loadAudit(auditId: string): Promise<AuditSnapshot>
  subscribe(auditId: string, onEvent: (event: AuditEvent) => void): () => void
}
