import { ChevronUp, RotateCcw } from "lucide-react"
import type { AuditCounts, AuditEvent } from "../../types/audit"

interface QueueStripProps {
  counts: AuditCounts
  events: readonly AuditEvent[]
  open: boolean
  onToggle: () => void
}

const eventLabel = (event: AuditEvent) =>
  `${event.packageId} moved to ${event.status.replaceAll("_", " ")}`

export function QueueStrip({ counts, events, open, onToggle }: QueueStripProps) {
  const active = counts.pending + counts.running + counts.retrying
  const stateLabel = active > 0 ? "Audit in progress" : "Audit complete"

  return (
    <footer className={`queue-strip ${open ? "queue-strip--open" : ""}`} aria-label="Queue activity">
      <div className="queue-strip__main">
        <div className="queue-state"><span className={active > 0 ? "activity-spinner" : "live-dot"} aria-hidden="true" />{stateLabel}</div>
        <div className="queue-strip__counts" aria-label="Queue counts">
          <span><b>{counts.pending}</b> pending</span>
          <span><b>{counts.running}</b> running</span>
          <span><b>{counts.retrying}</b> retrying</span>
          <span><b>{counts.completed}</b> completed</span>
          <span><b>{counts.dead_lettered}</b> DLQ</span>
        </div>
        <button className="button button--quiet" type="button" onClick={onToggle} aria-expanded={open}>
          <RotateCcw size={15} aria-hidden="true" />
          Recent activity
          <ChevronUp size={15} aria-hidden="true" className={open ? "" : "queue-strip__chevron"} />
        </button>
      </div>
      {open && (
        <div className="queue-events">
          {events.length === 0 ? (
            <p>No new lifecycle events in this run.</p>
          ) : (
            <ol>
              {events.map((event) => (
                <li key={event.id}>
                  <span>{eventLabel(event)}</span>
                  <time dateTime={event.occurredAt}>{new Date(event.occurredAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })}</time>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}
    </footer>
  )
}
