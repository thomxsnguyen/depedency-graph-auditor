import sampleAudit from "./sample-audit.json"
import type { AuditDataSource } from "./AuditDataSource"
import { countStatuses, type AuditEvent, type AuditSnapshot } from "../types/audit"

const fixtureEvents: AuditEvent[] = [
  {
    id: "demo-event-1",
    type: "package.status",
    occurredAt: "2026-08-31T18:00:01Z",
    packageId: "unstable-parser@2.0.1",
    status: "running",
    attempts: 3,
  },
  {
    id: "demo-event-2",
    type: "package.status",
    occurredAt: "2026-08-31T18:00:02Z",
    packageId: "unstable-parser@2.0.1",
    status: "completed",
    attempts: 3,
    latestError: "",
  },
]

function cloneSnapshot(): AuditSnapshot {
  const fixture = structuredClone(sampleAudit) as Omit<AuditSnapshot, "counts">
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
    const timers = fixtureEvents.map((event, index) =>
      window.setTimeout(() => onEvent(structuredClone(event)), 900 + index * 900),
    )

    return () => timers.forEach((timer) => window.clearTimeout(timer))
  }
}
