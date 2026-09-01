import { useState } from "react"
import { MessageSquareText, X } from "lucide-react"
import type { AuditPackage, DependencyEdge } from "../../types/audit"
import { StatusIndicator } from "../primitives/StatusIndicator"

interface NodeInspectorProps {
  root: string
  packageRow: AuditPackage | null
  path: string[]
  edges: readonly DependencyEdge[]
  annotation: string
  open: boolean
  saveStatus: "idle" | "saving" | "saved" | "error"
  onClose: () => void
  onAnnotate: (text: string) => void
}

const displayId = (id: string, root: string) => id === "root" ? root : id

export function NodeInspector({
  root,
  packageRow,
  path,
  edges,
  annotation,
  open,
  saveStatus,
  onClose,
  onAnnotate,
}: NodeInspectorProps) {
  const [note, setNote] = useState(annotation)

  const parentCount = packageRow ? new Set(edges.filter((edge) => edge.to === packageRow.id).map((edge) => edge.from)).size : 0
  const childCount = packageRow ? new Set(edges.filter((edge) => edge.from === packageRow.id).map((edge) => edge.to)).size : 0

  return (
    <aside className={`sidebar sidebar--right ${open ? "sidebar--open" : ""}`} aria-label="Selected package inspector">
      <div className="sidebar__mobile-header">
        <strong>Package inspector</strong>
        <button className="icon-button" type="button" onClick={onClose} aria-label="Close package inspector">
          <X size={18} aria-hidden="true" />
        </button>
      </div>

      {!packageRow ? (
        <div className="inspector-empty">
          <span className="empty-icon"><MessageSquareText size={20} aria-hidden="true" /></span>
          <strong>Select a package</strong>
          <p>Inspect its license, job state, relationships, and shortest dependency path.</p>
        </div>
      ) : (
        <>
          <div>
            <p className="eyebrow">Selected package</p>
            <h2>{packageRow.name} <span>{packageRow.version}</span></h2>
          </div>

          <dl className="details-list">
            <div><dt>Status</dt><dd><StatusIndicator status={packageRow.status} verdict={packageRow.verdict} /></dd></div>
            <div><dt>License</dt><dd>{packageRow.license || "Unknown"}</dd></div>
            <div><dt>Parents</dt><dd>{parentCount}</dd></div>
            <div><dt>Children</dt><dd>{childCount}</dd></div>
            <div><dt>Attempts</dt><dd>{packageRow.attempts ?? 0} of 5</dd></div>
          </dl>

          {packageRow.latestError && (
            <div className="inspector-message inspector-message--error">
              <strong>Latest job message</strong>
              <p>{packageRow.latestError}</p>
            </div>
          )}

          <div className="path-block">
            <span>Shortest dependency path</span>
            <ol>
              {path.map((nodeId) => <li key={nodeId}>{displayId(nodeId, root)}</li>)}
            </ol>
          </div>

          <form
            className="annotation-form"
            onSubmit={(event) => {
              event.preventDefault()
              onAnnotate(note)
            }}
          >
            <label htmlFor="package-note">Presentation note</label>
            <textarea
              id="package-note"
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder="Add context to this saved view"
              rows={3}
            />
            <div>
              <span className={`save-status save-status--${saveStatus}`} aria-live="polite">
                {saveStatus === "saving" ? "Saving…" : saveStatus === "error" ? "Could not save locally" : saveStatus === "saved" ? "Saved locally" : "Local view only"}
              </span>
              <button className="button button--secondary" type="submit">Save note</button>
            </div>
          </form>
        </>
      )}
    </aside>
  )
}
