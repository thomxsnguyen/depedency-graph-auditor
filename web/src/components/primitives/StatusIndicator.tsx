import type { AuditStatus, AuditVerdict } from "../../types/audit"

const statusLabels: Record<AuditStatus, string> = {
  pending: "Pending",
  running: "Running",
  retrying: "Retrying",
  completed: "Completed",
  dead_lettered: "Dead-lettered",
}

interface StatusIndicatorProps {
  status: AuditStatus
  verdict?: AuditVerdict
  compact?: boolean
}

export function StatusIndicator({ status, verdict, compact = false }: StatusIndicatorProps) {
  const visualState = verdict === "policy_violation" ? "violation" : status
  const label = verdict === "policy_violation" ? "Policy violation" : statusLabels[status]

  return (
    <span className={`status-indicator status-indicator--${visualState}`}>
      <i aria-hidden="true" />
      {!compact && <span>{label}</span>}
      {compact && <span className="sr-only">{label}</span>}
    </span>
  )
}
