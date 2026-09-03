import { FileText, PanelLeft, PanelRight, Play, RotateCcw } from "lucide-react"
import type { GraphMode } from "../../types/fileGraph"
import { GraphModeSelector } from "../GraphModeSelector/GraphModeSelector"

interface TopBarProps {
  root: string
  source: string
  reportUrl?: string
  running: boolean
  onRun: () => void
  onOpenSummary: () => void
  onOpenInspector: () => void
  hasSelection: boolean
  graphMode: GraphMode
  onGraphModeChange: (mode: GraphMode) => void
  summaryButtonRef: React.RefObject<HTMLButtonElement | null>
  inspectorButtonRef: React.RefObject<HTMLButtonElement | null>
}

export function TopBar({
  root,
  source,
  reportUrl,
  running,
  onRun,
  onOpenSummary,
  onOpenInspector,
  hasSelection,
  graphMode,
  onGraphModeChange,
  summaryButtonRef,
  inspectorButtonRef,
}: TopBarProps) {
  return (
    <header className="topbar">
      <div className="topbar__identity">
        <p className="eyebrow">Dependency Audit Studio</p>
        <h1>{root}</h1>
        <span className="source-label source-label--mobile">{source}</span>
      </div>

      <div className="topbar__mode">
        <GraphModeSelector mode={graphMode} onChange={onGraphModeChange} />
      </div>

      <div className="topbar__actions">
        <span className="source-label">{source}</span>
        <button
          ref={summaryButtonRef}
          className="icon-button mobile-panel-trigger"
          type="button"
          onClick={onOpenSummary}
          aria-label={graphMode === "dependencies" ? "Open audit summary and filters" : "Open file graph summary"}
        >
          <PanelLeft size={18} aria-hidden="true" />
        </button>
        <button
          ref={inspectorButtonRef}
          className="icon-button mobile-panel-trigger"
          type="button"
          onClick={onOpenInspector}
          aria-label={graphMode === "dependencies" ? "Open selected package inspector" : "Open selected file inspector"}
          disabled={!hasSelection}
        >
          <PanelRight size={18} aria-hidden="true" />
        </button>
        {reportUrl && (
          <a className="button button--secondary topbar__report" href={reportUrl} target="_blank" rel="noreferrer">
            <FileText size={16} aria-hidden="true" />
            View report
          </a>
        )}
        <button className="button button--primary" type="button" onClick={onRun} disabled={running}>
          {running ? <RotateCcw size={16} aria-hidden="true" /> : <Play size={16} aria-hidden="true" />}
          {running ? "Running demo" : "Run demo"}
        </button>
      </div>
    </header>
  )
}
