import sampleAudit from "../src/data/sample-audit.json"
import { countStatuses, type AuditSnapshot, type PackageStatusEvent } from "../src/types/audit"
import { auditReducer, createAuditState } from "../src/state/auditReducer"

const snapshot: AuditSnapshot = {
  ...(sampleAudit as Omit<AuditSnapshot, "counts">),
  counts: countStatuses(sampleAudit.packages as AuditSnapshot["packages"]),
}

describe("auditReducer", () => {
  it("applies lifecycle events without mutating the snapshot", () => {
    const before = structuredClone(snapshot)
    const state = createAuditState(snapshot)
    const event: PackageStatusEvent = {
      id: "event-1",
      type: "package.status",
      occurredAt: "2026-08-31T18:00:00Z",
      packageId: "unstable-parser@2.0.1",
      status: "completed",
      attempts: 3,
    }

    const next = auditReducer(state, { type: "event", event })!
    expect(next.snapshot.packages.find((row) => row.id === event.packageId)?.status).toBe("completed")
    expect(next.snapshot.counts.retrying).toBe(0)
    expect(snapshot).toEqual(before)
  })

  it("ignores duplicate event IDs", () => {
    const event: PackageStatusEvent = {
      id: "event-duplicate",
      type: "package.status",
      occurredAt: "2026-08-31T18:00:00Z",
      packageId: "unstable-parser@2.0.1",
      status: "running",
    }
    const once = auditReducer(createAuditState(snapshot), { type: "event", event })!
    const twice = auditReducer(once, { type: "event", event })!

    expect(twice).toBe(once)
    expect(twice.recentEvents).toHaveLength(1)
  })
})
