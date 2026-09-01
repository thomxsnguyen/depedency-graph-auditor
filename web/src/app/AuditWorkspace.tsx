import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react"
import type { Viewport } from "@xyflow/react"
import { AuditSidebar } from "../components/AuditSidebar/AuditSidebar"
import { GraphCanvas } from "../components/GraphCanvas/GraphCanvas"
import { NodeInspector } from "../components/NodeInspector/NodeInspector"
import { QueueStrip } from "../components/QueueStrip/QueueStrip"
import { TopBar } from "../components/TopBar/TopBar"
import { FixtureAuditDataSource } from "../data/FixtureAuditDataSource"
import { LocalGraphViewStore } from "../data/LocalGraphViewStore"
import { findPath } from "../graph/graphSelectors"
import { auditReducer } from "../state/auditReducer"
import { createHistory, historyReducer } from "../state/historyReducer"
import { viewReducer, type ViewAction } from "../state/viewReducer"
import type { GraphPosition, GraphView } from "../types/graphView"
import { initialGraphView } from "../types/graphView"

const auditId = "classic-demo"
const dataSource = new FixtureAuditDataSource()
const viewStore = new LocalGraphViewStore()
const reduceViewHistory = historyReducer<GraphView, ViewAction>(viewReducer)

export function AuditWorkspace() {
  const [auditState, auditDispatch] = useReducer(auditReducer, null)
  const [viewHistory, viewDispatch] = useReducer(reduceViewHistory, createHistory(initialGraphView))
  const [loadError, setLoadError] = useState<string | null>(null)
  const [viewHydrated, setViewHydrated] = useState(false)
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle")
  const [runToken, setRunToken] = useState(0)
  const summaryButtonRef = useRef<HTMLButtonElement>(null)
  const inspectorButtonRef = useRef<HTMLButtonElement>(null)
  const view = viewHistory.present
  const auditLoaded = auditState !== null

  const applyView = useCallback((action: ViewAction) => {
    viewDispatch({ type: "apply", action })
  }, [])

  useEffect(() => {
    let active = true

    Promise.all([dataSource.loadAudit(auditId), viewStore.load(auditId)])
      .then(([snapshot, savedView]) => {
        if (!active) return
        auditDispatch({ type: "reset", snapshot })
        if (savedView) viewDispatch({ type: "replace", value: savedView })
        setViewHydrated(true)
        setRunToken((value) => value + 1)
      })
      .catch(() => {
        if (active) setLoadError("The local audit fixture could not be loaded.")
      })

    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    if (!auditLoaded || runToken === 0) return
    return dataSource.subscribe(auditId, (event) => auditDispatch({ type: "event", event }))
  }, [runToken, auditLoaded])

  useEffect(() => {
    if (!viewHydrated) return
    const timer = window.setTimeout(() => {
      setSaveStatus("saving")
      viewStore.save(auditId, view)
        .then(() => setSaveStatus("saved"))
        .catch(() => setSaveStatus("error"))
    }, 280)
    return () => window.clearTimeout(timer)
  }, [view, viewHydrated])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return
      if (view.summaryOpen) {
        applyView({ type: "panel", panel: "summaryOpen", value: false })
        summaryButtonRef.current?.focus()
      }
      if (view.inspectorOpen) {
        applyView({ type: "panel", panel: "inspectorOpen", value: false })
        inspectorButtonRef.current?.focus()
      }
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [applyView, view.inspectorOpen, view.summaryOpen])

  const snapshot = auditState?.snapshot
  const selectedPackage = useMemo(
    () => snapshot?.packages.find((packageRow) => packageRow.id === view.selectedNodeId) ?? null,
    [snapshot, view.selectedNodeId],
  )
  const selectedPath = useMemo(
    () => snapshot && view.selectedNodeId ? findPath(view.selectedNodeId, snapshot.edges) : [],
    [snapshot, view.selectedNodeId],
  )
  const violationCount = snapshot?.packages.filter((packageRow) => packageRow.verdict === "policy_violation").length ?? 0
  const activeCount = snapshot ? snapshot.counts.pending + snapshot.counts.running + snapshot.counts.retrying : 0

  const runDemo = useCallback(() => {
    setLoadError(null)
    dataSource.loadAudit(auditId)
      .then((nextSnapshot) => {
        auditDispatch({ type: "reset", snapshot: nextSnapshot })
        setRunToken((value) => value + 1)
      })
      .catch(() => setLoadError("The local audit fixture could not be restarted."))
  }, [])

  const closeSummary = useCallback(() => {
    applyView({ type: "panel", panel: "summaryOpen", value: false })
    window.requestAnimationFrame(() => summaryButtonRef.current?.focus())
  }, [applyView])

  const closeInspector = useCallback(() => {
    applyView({ type: "panel", panel: "inspectorOpen", value: false })
    window.requestAnimationFrame(() => inspectorButtonRef.current?.focus())
  }, [applyView])

  const selectNode = useCallback((nodeId: string) => {
    applyView({ type: "select", nodeId })
    if (nodeId !== "root" && window.matchMedia("(max-width: 1199px)").matches) {
      applyView({ type: "panel", panel: "inspectorOpen", value: true })
    }
  }, [applyView])

  const positionNode = useCallback((nodeId: string, position: GraphPosition) => {
    applyView({ type: "position", nodeId, position })
  }, [applyView])

  const saveViewport = useCallback((viewport: Viewport) => {
    applyView({ type: "viewport", viewport })
  }, [applyView])

  if (loadError) {
    return (
      <main className="centered-state" role="alert">
        <div className="state-panel">
          <p className="eyebrow">Dependency Audit Studio</p>
          <h1>The demo could not be loaded.</h1>
          <p>{loadError}</p>
          <button className="button button--primary" type="button" onClick={runDemo}>Try again</button>
        </div>
      </main>
    )
  }

  if (!auditState || !snapshot) {
    return (
      <main className="centered-state" aria-busy="true">
        <div className="state-panel state-panel--loading">
          <span className="loading-mark" aria-hidden="true" />
          <p className="eyebrow">Dependency Audit Studio</p>
          <h1>Preparing the local audit…</h1>
          <p>The dependency graph and queue state will appear here.</p>
        </div>
      </main>
    )
  }

  return (
    <main className="app-shell">
      <TopBar
        root={snapshot.root}
        source={snapshot.source}
        reportUrl={snapshot.reportUrl}
        running={activeCount > 0}
        onRun={runDemo}
        onOpenSummary={() => applyView({ type: "panel", panel: "summaryOpen", value: true })}
        onOpenInspector={() => applyView({ type: "panel", panel: "inspectorOpen", value: true })}
        hasSelection={Boolean(selectedPackage)}
        summaryButtonRef={summaryButtonRef}
        inspectorButtonRef={inspectorButtonRef}
      />

      <section className="workspace" aria-label="Dependency audit workspace">
        <AuditSidebar
          packageCount={snapshot.packages.length}
          violationCount={violationCount}
          counts={snapshot.counts}
          filters={view.filters}
          open={view.summaryOpen}
          onClose={closeSummary}
          onSearch={(value) => applyView({ type: "search", value })}
          onFilter={(name, value) => applyView({ type: "filter", name, value })}
        />

        <GraphCanvas
          snapshot={snapshot}
          view={view}
          canUndo={viewHistory.past.length > 0}
          canRedo={viewHistory.future.length > 0}
          onSelect={selectNode}
          onCollapse={(nodeId) => applyView({ type: "collapse", nodeId })}
          onPosition={positionNode}
          onViewport={saveViewport}
          onUndo={() => viewDispatch({ type: "undo" })}
          onRedo={() => viewDispatch({ type: "redo" })}
          onResetLayout={() => applyView({ type: "reset-layout" })}
        />

        <NodeInspector
          key={`${selectedPackage?.id ?? "none"}:${selectedPackage ? view.annotations[selectedPackage.id] ?? "" : ""}`}
          root={snapshot.root}
          packageRow={selectedPackage}
          path={selectedPath}
          edges={snapshot.edges}
          annotation={selectedPackage ? view.annotations[selectedPackage.id] ?? "" : ""}
          open={view.inspectorOpen}
          saveStatus={saveStatus}
          onClose={closeInspector}
          onAnnotate={(text) => selectedPackage && applyView({ type: "annotate", nodeId: selectedPackage.id, text })}
        />

        {(view.summaryOpen || view.inspectorOpen) && (
          <button
            className="panel-scrim"
            type="button"
            aria-label="Close open panel"
            onClick={view.summaryOpen ? closeSummary : closeInspector}
          />
        )}
      </section>

      <QueueStrip
        counts={snapshot.counts}
        events={auditState.recentEvents}
        open={view.activityOpen}
        onToggle={() => applyView({ type: "panel", panel: "activityOpen", value: !view.activityOpen })}
      />

      <div className="sr-only" aria-live="polite" aria-atomic="true">
        {activeCount > 0
          ? `${activeCount} dependency jobs remain active.`
          : `Audit complete. ${snapshot.counts.completed} jobs completed and ${snapshot.counts.dead_lettered} jobs were dead-lettered.`}
      </div>
    </main>
  )
}
