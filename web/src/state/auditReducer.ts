import { countStatuses, type AuditEvent, type AuditSnapshot } from "../types/audit"

export interface AuditState {
  snapshot: AuditSnapshot
  processedEventIds: readonly string[]
  recentEvents: readonly AuditEvent[]
}

export type AuditAction =
  | { type: "reset"; snapshot: AuditSnapshot }
  | { type: "event"; event: AuditEvent }

export function createAuditState(snapshot: AuditSnapshot): AuditState {
  return {
    snapshot,
    processedEventIds: [],
    recentEvents: [],
  }
}

export function auditReducer(state: AuditState | null, action: AuditAction): AuditState | null {
  if (action.type === "reset") return createAuditState(action.snapshot)
  if (!state) return state
  if (state.processedEventIds.includes(action.event.id)) return state

  const packages = state.snapshot.packages.map((packageRow) =>
    packageRow.id === action.event.packageId
      ? {
          ...packageRow,
          status: action.event.status,
          attempts: action.event.attempts ?? packageRow.attempts,
          latestError: action.event.latestError ?? packageRow.latestError,
          verdict: action.event.verdict ?? packageRow.verdict,
        }
      : packageRow,
  )

  return {
    snapshot: {
      ...state.snapshot,
      packages,
      counts: countStatuses(packages),
    },
    processedEventIds: [...state.processedEventIds, action.event.id],
    recentEvents: [action.event, ...state.recentEvents].slice(0, 8),
  }
}
