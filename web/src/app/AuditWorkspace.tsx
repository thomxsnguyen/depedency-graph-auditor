import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react"
import type { Viewport } from "@xyflow/react"
import { AuditSidebar } from "../components/AuditSidebar/AuditSidebar"
import { GraphCanvas } from "../components/GraphCanvas/GraphCanvas"
import { FileGraphCanvas } from "../components/FileGraphCanvas/FileGraphCanvas"
import { FileInspector } from "../components/FileInspector/FileInspector"
import { FileSidebar } from "../components/FileSidebar/FileSidebar"
import { NodeInspector } from "../components/NodeInspector/NodeInspector"
import { TopBar } from "../components/TopBar/TopBar"
import { FixtureAuditDataSource } from "../data/FixtureAuditDataSource"
import { FixtureFileGraphDataSource } from "../data/FixtureFileGraphDataSource"
import {
  HttpRepositoryFileGraphDataSource,
  type GitHubFileGraphRequest,
} from "../data/RepositoryFileGraphDataSource"
import { LocalGraphViewStore } from "../data/LocalGraphViewStore"
import { findPath } from "../graph/graphSelectors"
import {
  architectureEntityId,
  architectureForFile,
  domainEntityId,
  type DependencyHopScope,
  type ExpandedArchitectureItem,
} from "../graph/hierarchicalFileGraph"
import {
  diagnosticsForFile,
  fileGraphCounts,
  incomingFiles,
  outgoingFiles,
  selectedFile,
} from "../graph/fileGraphSelectors"
import { auditReducer } from "../state/auditReducer"
import { createHistory, historyReducer } from "../state/historyReducer"
import { viewReducer, type ViewAction } from "../state/viewReducer"
import type { GraphPosition, GraphView } from "../types/graphView"
import { initialGraphView } from "../types/graphView"
import type { FileGraphSnapshot, GraphMode } from "../types/fileGraph"

const auditId = "classic-demo"
const dataSource = new FixtureAuditDataSource()
const fileGraphDataSource = new FixtureFileGraphDataSource()
const repositoryFileGraphDataSource = new HttpRepositoryFileGraphDataSource()
const viewStore = new LocalGraphViewStore()
const reduceViewHistory = historyReducer<GraphView, ViewAction>(viewReducer)

export function AuditWorkspace() {
  const [auditState, auditDispatch] = useReducer(auditReducer, null)
  const [viewHistory, viewDispatch] = useReducer(reduceViewHistory, createHistory(initialGraphView))
  const [loadError, setLoadError] = useState<string | null>(null)
  const [viewHydrated, setViewHydrated] = useState(false)
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle")
  const [runToken, setRunToken] = useState(0)
  const [graphMode, setGraphMode] = useState<GraphMode>("dependencies")
  const [fileGraph, setFileGraph] = useState<FileGraphSnapshot | null>(null)
  const [fileGraphStatus, setFileGraphStatus] = useState<"loading" | "ready" | "error">("loading")
  const [fileSearch, setFileSearch] = useState("")
  const [selectedFilePath, setSelectedFilePath] = useState<string | null>(null)
  const [selectedFileGroupId, setSelectedFileGroupId] = useState<string | null>(null)
  const [expandedArchitectureIds, setExpandedArchitectureIds] = useState<ReadonlySet<string>>(() => new Set())
  const [expandedDomainIds, setExpandedDomainIds] = useState<ReadonlySet<string>>(() => new Set())
  const [fileHopScope, setFileHopScope] = useState<DependencyHopScope>(1)
  const [filePositions, setFilePositions] = useState<Record<string, GraphPosition>>({})
  const [fileViewport, setFileViewport] = useState<Viewport | null>(null)
  const [repositoryStatus, setRepositoryStatus] = useState<"idle" | "submitting" | "success" | "error">("idle")
  const [repositoryError, setRepositoryError] = useState<string | null>(null)
  const repositoryRequestRef = useRef<AbortController | null>(null)
  const summaryButtonRef = useRef<HTMLButtonElement>(null)
  const inspectorButtonRef = useRef<HTMLButtonElement>(null)
  const view = viewHistory.present
  const auditLoaded = auditState !== null

  const loadFileGraph = useCallback(() => {
    return fileGraphDataSource.load()
      .then((snapshot) => {
        setFileGraph(snapshot)
        setFileGraphStatus("ready")
      })
      .catch(() => {
        setFileGraph(null)
        setFileGraphStatus("error")
      })
  }, [])

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
    void loadFileGraph()
  }, [loadFileGraph])

  useEffect(() => () => repositoryRequestRef.current?.abort(), [])

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
  const selectedFileNode = useMemo(
    () => fileGraph ? selectedFile(fileGraph, selectedFilePath) : null,
    [fileGraph, selectedFilePath],
  )
  const selectedFileIncoming = useMemo(
    () => fileGraph && selectedFilePath ? incomingFiles(fileGraph, selectedFilePath) : [],
    [fileGraph, selectedFilePath],
  )
  const selectedFileOutgoing = useMemo(
    () => fileGraph && selectedFilePath ? outgoingFiles(fileGraph, selectedFilePath) : [],
    [fileGraph, selectedFilePath],
  )
  const selectedFileDiagnostics = useMemo(
    () => fileGraph && selectedFilePath ? diagnosticsForFile(fileGraph, selectedFilePath) : [],
    [fileGraph, selectedFilePath],
  )
  const fileCounts = useMemo(
    () => fileGraph ? fileGraphCounts(fileGraph) : { files: 0, imports: 0, diagnostics: 0 },
    [fileGraph],
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

  const selectFileNode = useCallback((path: string | null) => {
    setSelectedFilePath(path)
    setSelectedFileGroupId(null)
    if (path) setFileHopScope(1)
    if (path && window.matchMedia("(max-width: 1199px)").matches) {
      applyView({ type: "panel", panel: "inspectorOpen", value: true })
    }
  }, [applyView])

  const selectFileGroup = useCallback((id: string | null) => {
    setSelectedFileGroupId(id)
    setSelectedFilePath(null)
    setFileHopScope(1)
    if (view.inspectorOpen) applyView({ type: "panel", panel: "inspectorOpen", value: false })
  }, [applyView, view.inspectorOpen])

  const clearHiddenFileSelection = useCallback((nextArchitectureIds: ReadonlySet<string>, nextDomainIds: ReadonlySet<string>) => {
    if (!selectedFilePath) return
    const architecture = architectureForFile(selectedFilePath)
    const architectureId = architectureEntityId(architecture.project, architecture.layer)
    const domainId = domainEntityId(architecture.project, architecture.layer, architecture.domain)
    if (!nextArchitectureIds.has(architectureId) || !nextDomainIds.has(domainId)) {
      setSelectedFilePath(null)
      setFileHopScope(1)
      if (view.inspectorOpen) applyView({ type: "panel", panel: "inspectorOpen", value: false })
    }
  }, [applyView, selectedFilePath, view.inspectorOpen])

  const toggleArchitecture = useCallback((id: string, project: string) => {
    const nextArchitectureIds = new Set(expandedArchitectureIds)
    const nextDomainIds = new Set(expandedDomainIds)
    const collapsing = nextArchitectureIds.has(id)
    const architectureById = new Map((fileGraph?.nodes ?? []).map((node) => {
      const architecture = architectureForFile(node.path)
      return [architectureEntityId(architecture.project, architecture.layer), architecture] as const
    }))
    for (const expandedId of nextArchitectureIds) {
      if (expandedId === id || (!collapsing && architectureById.get(expandedId)?.project === project)) {
        nextArchitectureIds.delete(expandedId)
        for (const node of fileGraph?.nodes ?? []) {
          const architecture = architectureForFile(node.path)
          if (architectureEntityId(architecture.project, architecture.layer) === expandedId) {
            nextDomainIds.delete(domainEntityId(architecture.project, architecture.layer, architecture.domain))
          }
        }
      }
    }
    if (!collapsing) nextArchitectureIds.add(id)
    setExpandedArchitectureIds(nextArchitectureIds)
    setExpandedDomainIds(nextDomainIds)
    setSelectedFileGroupId(null)
    clearHiddenFileSelection(nextArchitectureIds, nextDomainIds)
  }, [clearHiddenFileSelection, expandedArchitectureIds, expandedDomainIds, fileGraph?.nodes])

  const toggleDomain = useCallback((id: string, architectureId: string) => {
    const nextDomainIds = new Set(expandedDomainIds)
    const collapsing = nextDomainIds.has(id)
    for (const expandedId of nextDomainIds) {
      const belongsToArchitecture = (fileGraph?.nodes ?? []).some((node) => {
        const architecture = architectureForFile(node.path)
        return domainEntityId(architecture.project, architecture.layer, architecture.domain) === expandedId
          && architectureEntityId(architecture.project, architecture.layer) === architectureId
      })
      if (expandedId === id || (!collapsing && belongsToArchitecture)) nextDomainIds.delete(expandedId)
    }
    if (!collapsing) nextDomainIds.add(id)
    setExpandedDomainIds(nextDomainIds)
    setSelectedFileGroupId(null)
    clearHiddenFileSelection(expandedArchitectureIds, nextDomainIds)
  }, [clearHiddenFileSelection, expandedArchitectureIds, expandedDomainIds, fileGraph?.nodes])

  const collapseFileExpansion = useCallback((item: ExpandedArchitectureItem) => {
    for (const node of fileGraph?.nodes ?? []) {
      const architecture = architectureForFile(node.path)
      const architectureId = architectureEntityId(architecture.project, architecture.layer)
      if (item.kind === "architecture" && architectureId === item.id) {
        toggleArchitecture(item.id, architecture.project)
        return
      }
      if (item.kind === "domain" && domainEntityId(architecture.project, architecture.layer, architecture.domain) === item.id) {
        toggleDomain(item.id, architectureId)
        return
      }
    }
  }, [fileGraph?.nodes, toggleArchitecture, toggleDomain])

  const changeGraphMode = useCallback((mode: GraphMode) => {
    setGraphMode(mode)
    const hasModeSelection = mode === "dependencies" ? Boolean(selectedPackage) : Boolean(selectedFileNode)
    if (!hasModeSelection && view.inspectorOpen) {
      applyView({ type: "panel", panel: "inspectorOpen", value: false })
    }
  }, [applyView, selectedFileNode, selectedPackage, view.inspectorOpen])

  const analyzeRepository = useCallback((request: GitHubFileGraphRequest) => {
    if (repositoryRequestRef.current) return
    const controller = new AbortController()
    repositoryRequestRef.current = controller
    setRepositoryError(null)
    setRepositoryStatus("submitting")
    repositoryFileGraphDataSource.analyze(request, controller.signal)
      .then((nextGraph) => {
        setFileGraph(nextGraph)
        setFileGraphStatus("ready")
        setSelectedFilePath(null)
        setSelectedFileGroupId(null)
        setExpandedArchitectureIds(new Set())
        setExpandedDomainIds(new Set())
        setFileHopScope(1)
        setFileSearch("")
        setFilePositions({})
        setFileViewport(null)
        setRepositoryStatus("success")
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return
        setRepositoryError(error instanceof Error ? error.message : "The GitHub repository could not be analyzed.")
        setRepositoryStatus("error")
      })
      .finally(() => {
        if (repositoryRequestRef.current === controller) repositoryRequestRef.current = null
      })
  }, [])

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
        root={graphMode === "files" && fileGraph ? fileGraph.root : snapshot.root}
        source={graphMode === "dependencies" ? snapshot.source : "File dependencies · local demo"}
        reportUrl={snapshot.reportUrl}
        running={activeCount > 0}
        onRun={runDemo}
        onOpenSummary={() => applyView({ type: "panel", panel: "summaryOpen", value: true })}
        onOpenInspector={() => applyView({ type: "panel", panel: "inspectorOpen", value: true })}
        hasSelection={graphMode === "dependencies" ? Boolean(selectedPackage) : Boolean(selectedFileNode)}
        graphMode={graphMode}
        onGraphModeChange={changeGraphMode}
        summaryButtonRef={summaryButtonRef}
        inspectorButtonRef={inspectorButtonRef}
      />

      <section className="workspace" aria-label="Dependency audit workspace">
        {graphMode === "dependencies" ? (
          <>
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
              onViewAction={applyView}
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
          </>
        ) : (
          <>
            <FileSidebar
              fileCount={fileCounts.files}
              importCount={fileCounts.imports}
              diagnosticCount={fileCounts.diagnostics}
              search={fileSearch}
              open={view.summaryOpen}
              repositorySubmitting={repositoryStatus === "submitting"}
              repositoryError={repositoryError}
              onClose={closeSummary}
              onSearch={setFileSearch}
              onRepositorySubmit={analyzeRepository}
              onRepositoryChange={() => {
                setRepositoryError(null)
                if (repositoryStatus === "error") setRepositoryStatus("idle")
              }}
            />

            {fileGraphStatus === "loading" ? (
              <section className="graph-canvas" aria-label="File dependency graph canvas" aria-busy="true">
                <div className="graph-empty" role="status"><strong>Loading file graph…</strong></div>
              </section>
            ) : fileGraphStatus === "error" || !fileGraph ? (
              <section className="graph-canvas" aria-label="File dependency graph canvas">
                <div className="graph-empty" role="alert">
                  <strong>The file graph could not be displayed.</strong>
                  <button
                    className="button button--secondary"
                    type="button"
                    onClick={() => {
                      setFileGraphStatus("loading")
                      void loadFileGraph()
                    }}
                  >
                    Try again
                  </button>
                </div>
              </section>
            ) : (
              <FileGraphCanvas
                snapshot={fileGraph}
                search={fileSearch}
                selectedPath={selectedFilePath}
                selectedGroupId={selectedFileGroupId}
                expandedArchitectureIds={expandedArchitectureIds}
                expandedDomainIds={expandedDomainIds}
                hopScope={fileHopScope}
                positions={filePositions}
                viewport={fileViewport}
                onSelect={selectFileNode}
                onSelectGroup={selectFileGroup}
                onToggleArchitecture={toggleArchitecture}
                onToggleDomain={toggleDomain}
                onCollapseExpansion={collapseFileExpansion}
                onHopScope={setFileHopScope}
                onPosition={(id, position) => setFilePositions((positions) => ({ ...positions, [id]: position }))}
                onViewport={setFileViewport}
                onReset={() => {
                  setFilePositions({})
                  setFileViewport(null)
                }}
                onClearSearch={() => setFileSearch("")}
              />
            )}

            <FileInspector
              file={selectedFileNode}
              incoming={selectedFileIncoming}
              outgoing={selectedFileOutgoing}
              diagnostics={selectedFileDiagnostics}
              open={view.inspectorOpen}
              onClose={closeInspector}
            />
          </>
        )}

        {(view.summaryOpen || view.inspectorOpen) && (
          <button
            className="panel-scrim"
            type="button"
            aria-label="Close open panel"
            onClick={view.summaryOpen ? closeSummary : closeInspector}
          />
        )}
      </section>

      <div className="sr-only" aria-live="polite" aria-atomic="true">
        {graphMode === "files"
          ? `File dependency graph selected. ${fileCounts.files} files and ${fileCounts.imports} resolved imports.`
          : activeCount > 0
          ? `${activeCount} dependency jobs remain active.`
          : `Audit complete. ${snapshot.counts.completed} jobs completed and ${snapshot.counts.dead_lettered} jobs were dead-lettered.`}
      </div>
    </main>
  )
}
