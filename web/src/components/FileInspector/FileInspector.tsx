import { FileCode2, X } from "lucide-react"
import type { FileGraphDiagnostic, FileGraphNode } from "../../types/fileGraph"

interface FileInspectorProps {
  file: FileGraphNode | null
  incoming: readonly string[]
  outgoing: readonly string[]
  diagnostics: readonly FileGraphDiagnostic[]
  open: boolean
  onClose: () => void
}

function RelationshipList({ title, paths }: { title: string; paths: readonly string[] }) {
  return (
    <div className="file-relationship-list">
      <span>{title}</span>
      {paths.length === 0 ? <p>None</p> : <ul>{paths.map((path) => <li key={path}>{path}</li>)}</ul>}
    </div>
  )
}

export function FileInspector({ file, incoming, outgoing, diagnostics, open, onClose }: FileInspectorProps) {
  return (
    <aside className={`sidebar sidebar--right ${open ? "sidebar--open" : ""}`} aria-label="Selected file inspector">
      <div className="sidebar__mobile-header">
        <strong>File inspector</strong>
        <button className="icon-button" type="button" onClick={onClose} aria-label="Close file inspector">
          <X size={18} aria-hidden="true" />
        </button>
      </div>

      {!file ? (
        <div className="inspector-empty">
          <span className="empty-icon"><FileCode2 size={20} aria-hidden="true" /></span>
          <strong>Select a file</strong>
          <p>Inspect its incoming and outgoing file dependencies.</p>
        </div>
      ) : (
        <>
          <div className="file-inspector__heading">
            <p className="eyebrow">Selected file</p>
            <h2>{file.path.split("/").at(-1)}</h2>
            <p>{file.path}</p>
          </div>

          <dl className="details-list">
            <div><dt>Imported by</dt><dd>{incoming.length}</dd></div>
            <div><dt>Imports</dt><dd>{outgoing.length}</dd></div>
            <div><dt>Diagnostics</dt><dd>{diagnostics.length}</dd></div>
          </dl>

          <RelationshipList title="Imports" paths={outgoing} />
          <RelationshipList title="Imported by" paths={incoming} />

          {diagnostics.length > 0 && (
            <div className="file-diagnostics">
              <span>Diagnostics</span>
              <ul>
                {diagnostics.map((diagnostic, index) => (
                  <li key={`${diagnostic.path}:${diagnostic.import ?? ""}:${index}`}>
                    <strong>{diagnostic.import || "File analysis"}</strong>
                    <p>{diagnostic.message}</p>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}
    </aside>
  )
}
