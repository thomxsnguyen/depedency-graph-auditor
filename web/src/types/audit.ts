export type AuditStatus =
  | "pending"
  | "running"
  | "retrying"
  | "completed"
  | "dead_lettered"

export type AuditVerdict = "pass" | "policy_violation" | "unresolvable"

export interface AuditPackage {
  id: string
  name: string
  version: string
  license: string
  verdict: AuditVerdict
  status: AuditStatus
  attempts?: number
  latestError?: string
}

export interface DependencyEdge {
  id: string
  from: string
  to: string
}

export type AuditCounts = Record<AuditStatus, number>

export interface AuditSnapshot {
  auditId: string
  root: string
  source: string
  packages: AuditPackage[]
  edges: DependencyEdge[]
  counts: AuditCounts
  reportUrl?: string
}

export interface PackageStatusEvent {
  id: string
  type: "package.status"
  occurredAt: string
  packageId: string
  status: AuditStatus
  attempts?: number
  latestError?: string
  verdict?: AuditVerdict
}

export type AuditEvent = PackageStatusEvent

export function countStatuses(packages: readonly AuditPackage[]): AuditCounts {
  const counts: AuditCounts = {
    pending: 0,
    running: 0,
    retrying: 0,
    completed: 0,
    dead_lettered: 0,
  }

  for (const packageRow of packages) {
    counts[packageRow.status] += 1
  }

  return counts
}
