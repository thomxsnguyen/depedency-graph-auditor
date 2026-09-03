import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Focus, RotateCcw, ZoomIn, ZoomOut } from "lucide-react"
import {
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type NodeTypes,
  type Viewport,
} from "@xyflow/react"
import type { FileGraphSnapshot } from "../../types/fileGraph"
import type { GraphPosition } from "../../types/graphView"
import { layoutGraph } from "../../graph/layoutGraph"
import { mapFileGraph, type FileGraphFlowNode } from "../../graph/mapFileGraph"
import { fileMatchesSearch } from "../../graph/fileGraphSelectors"
import { categoryDetails, classifyFile, FILE_CATEGORY_DETAILS } from "../../graph/fileCategory"
import {
  buildHierarchicalFileGraph,
  type DependencyHopScope,
} from "../../graph/hierarchicalFileGraph"
import { FileNode } from "./FileNode"
import { ModuleNode } from "./ModuleNode"

const nodeTypes: NodeTypes = { file: FileNode, module: ModuleNode }

interface FileGraphCanvasProps {
  snapshot: FileGraphSnapshot
  search: string
  selectedPath: string | null
  selectedModulePath: string | null
  expandedModulePaths: ReadonlySet<string>
  hopScope: DependencyHopScope
  positions: Record<string, GraphPosition>
  viewport: Viewport | null
  onSelect: (path: string | null) => void
  onSelectModule: (path: string | null) => void
  onToggleModule: (path: string) => void
  onHopScope: (scope: DependencyHopScope) => void
  onPosition: (id: string, position: GraphPosition) => void
  onViewport: (viewport: Viewport) => void
  onReset: () => void
  onClearSearch: () => void
}

function FileCanvasControls({
  selectedPath,
  hopScope,
  onHopScope,
  onReset,
}: Pick<FileGraphCanvasProps, "selectedPath" | "hopScope" | "onHopScope" | "onReset">) {
  const flow = useReactFlow()
  return (
    <div className="graph-toolbar" aria-label="File graph controls">
      <button className="icon-button" type="button" onClick={() => flow.zoomOut()} aria-label="Zoom out">
        <ZoomOut size={17} aria-hidden="true" />
      </button>
      <button className="icon-button" type="button" onClick={() => flow.fitView({ padding: 0.2, duration: 220 })} aria-label="Fit file graph to view">
        <Focus size={17} aria-hidden="true" />
      </button>
      <button className="icon-button" type="button" onClick={() => flow.zoomIn()} aria-label="Zoom in">
        <ZoomIn size={17} aria-hidden="true" />
      </button>
      <span className="toolbar-divider" aria-hidden="true" />
      <button className="icon-button" type="button" onClick={onReset} aria-label="Reset file graph layout">
        <RotateCcw size={17} aria-hidden="true" />
      </button>
      {selectedPath && (
        <>
          <span className="toolbar-divider" aria-hidden="true" />
          <div className="hop-scope" role="group" aria-label="Dependency hop scope">
            {([1, 2, "all"] as const).map((scope) => (
              <button
                key={scope}
                type="button"
                aria-pressed={hopScope === scope}
                onClick={() => onHopScope(scope)}
              >
                {scope === "all" ? "All" : `${scope} hop${scope === 2 ? "s" : ""}`}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

function ExpandedModuleControls({
  modulePaths,
  onToggle,
}: { modulePaths: ReadonlySet<string>; onToggle: (path: string) => void }) {
  if (modulePaths.size === 0) return null
  return (
    <aside className="expanded-modules" aria-label="Expanded modules">
      <span>Expanded</span>
      {[...modulePaths].sort().map((path) => (
        <button key={path} type="button" onClick={() => onToggle(path)} aria-label={`Collapse module ${path}`}>
          {path === "." ? "Repository root" : path}
          <i aria-hidden="true">×</i>
        </button>
      ))}
    </aside>
  )
}

function FileCategoryLegend({ paths }: { paths: readonly string[] }) {
  const categories = new Set(paths.map(classifyFile))
  return (
    <aside className="file-category-legend" aria-label="File category legend">
      <span className="file-category-legend__title">File types</span>
      <ul>
        {FILE_CATEGORY_DETAILS.filter(({ category }) => categories.has(category)).map(({ category, label }) => (
          <li key={category}>
            <i className={`file-category-swatch file-category-swatch--${category}`} aria-hidden="true" />
            <span>{label}</span>
          </li>
        ))}
      </ul>
    </aside>
  )
}

function FileGraphCanvasInner(props: FileGraphCanvasProps) {
  const flow = useReactFlow()
  const onReset = props.onReset
  const [layoutRequest, setLayoutRequest] = useState(0)
  const initialViewportRef = useRef(props.viewport)
  const completedInitialLayoutRef = useRef(false)
  const positionsRef = useRef(props.positions)

  useEffect(() => {
    positionsRef.current = props.positions
  }, [props.positions])

  const visibleGraph = useMemo(
    () => buildHierarchicalFileGraph(
      props.snapshot,
      props.expandedModulePaths,
      props.selectedPath,
      props.hopScope,
      props.search,
    ),
    [props.expandedModulePaths, props.hopScope, props.search, props.selectedPath, props.snapshot],
  )
  const mapped = useMemo(
    () => mapFileGraph(visibleGraph, props.selectedPath, props.selectedModulePath, props.search, props.positions),
    [props.positions, props.search, props.selectedModulePath, props.selectedPath, visibleGraph],
  )
  const interactiveNodes = useMemo(
    () => mapped.nodes.map((node): FileGraphFlowNode => node.type === "module"
      ? {
          ...node,
          data: { ...node.data, onSelect: props.onSelectModule, onToggle: props.onToggleModule },
        }
      : {
          ...node,
          data: { ...node.data, onSelect: props.onSelect },
        }),
    [mapped.nodes, props.onSelect, props.onSelectModule, props.onToggleModule],
  )
  const [nodes, setNodes, onNodesChange] = useNodesState<FileGraphFlowNode>(interactiveNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(mapped.edges)

  useEffect(() => {
    setEdges(mapped.edges)
    setNodes((currentNodes) => {
      const current = new Map(currentNodes.map((node) => [node.id, node]))
      return interactiveNodes.map((node) => ({
        ...node,
        position: props.positions[node.id] ?? current.get(node.id)?.position ?? node.position,
        selected: node.type === "file"
          ? node.data.path === props.selectedPath
          : node.data.path === props.selectedModulePath,
      }))
    })
  }, [interactiveNodes, mapped.edges, props.positions, props.selectedModulePath, props.selectedPath, setEdges, setNodes])

  const layoutNodes = useMemo(
    () => visibleGraph.nodes.map((node) => ({ id: node.id, width: 184, height: node.kind === "module" ? 88 : 68 })),
    [visibleGraph.nodes],
  )
  const layoutEdges = useMemo(
    () => visibleGraph.edges.map((edge) => ({ id: edge.id, source: edge.from, target: edge.to })),
    [visibleGraph.edges],
  )

  useEffect(() => {
    let active = true
    let fitFrame: number | null = null
    layoutGraph(layoutNodes, layoutEdges)
      .then((layoutPositions) => {
        if (!active) return
        setNodes((currentNodes) => currentNodes.map((node) => ({
          ...node,
          position: positionsRef.current[node.id] ?? layoutPositions[node.id] ?? node.position,
        })))
        const shouldFit = completedInitialLayoutRef.current || initialViewportRef.current === null
        completedInitialLayoutRef.current = true
        if (shouldFit) {
          fitFrame = window.requestAnimationFrame(() => flow.fitView({ padding: 0.2, duration: 220 }))
        }
      })
      .catch(() => {
        // Deterministic fallback positions remain usable when layout fails.
      })
    return () => {
      active = false
      if (fitFrame !== null) window.cancelAnimationFrame(fitFrame)
    }
  }, [flow, layoutEdges, layoutNodes, layoutRequest, setNodes])

  const resetLayout = useCallback(() => {
    positionsRef.current = {}
    onReset()
    setLayoutRequest((request) => request + 1)
  }, [onReset])

  const clearSelection = () => {
    props.onSelect(null)
    props.onSelectModule(null)
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={(_, node) => {
        if (node.type === "module") props.onSelectModule(node.data.path)
        else props.onSelect(node.data.path)
      }}
      onPaneClick={clearSelection}
      onNodeDragStop={(_, node) => props.onPosition(node.id, node.position)}
      onMoveEnd={(_, viewport) => props.onViewport(viewport)}
      defaultViewport={props.viewport ?? { x: 0, y: 0, zoom: 1 }}
      minZoom={nodes.length > 80 ? 0.05 : 0.25}
      maxZoom={1.8}
      nodesConnectable={false}
      nodesDraggable
      elementsSelectable
      deleteKeyCode={null}
      aria-label="Interactive file dependency graph"
    >
      <FileCanvasControls
        selectedPath={props.selectedPath}
        hopScope={props.hopScope}
        onHopScope={props.onHopScope}
        onReset={resetLayout}
      />
      <ExpandedModuleControls modulePaths={props.expandedModulePaths} onToggle={props.onToggleModule} />
      <FileCategoryLegend paths={props.snapshot.nodes.map((node) => node.path)} />
      <MiniMap
        className="graph-minimap"
        pannable
        zoomable
        nodeColor={(node) => node.data.entityKind === "module"
          ? "#62666c"
          : categoryDetails(classifyFile(typeof node.data.path === "string" ? node.data.path : "")).color}
        nodeStrokeColor="#d2d2d7"
        maskColor="rgba(245,245,247,0.78)"
        ariaLabel="File dependency graph minimap"
      />
    </ReactFlow>
  )
}

export function FileGraphCanvas(props: FileGraphCanvasProps) {
  if (props.snapshot.nodes.length === 0) {
    return (
      <section className="graph-canvas" aria-label="File dependency graph canvas">
        <div className="graph-empty" role="status">
          <strong>No file dependencies found.</strong>
          <span>The supplied file graph does not contain any files.</span>
        </div>
      </section>
    )
  }

  const hasMatches = props.snapshot.nodes.some((node) => fileMatchesSearch(node.path, props.search))
  if (props.search.trim() && !hasMatches) {
    return (
      <section className="graph-canvas" aria-label="File dependency graph canvas">
        <div className="graph-empty" role="status">
          <strong>No files match this search.</strong>
          <button className="button button--secondary" type="button" onClick={props.onClearSearch}>Clear search</button>
        </div>
      </section>
    )
  }

  return (
    <section className="graph-canvas" aria-label="File dependency graph canvas">
      <ReactFlowProvider>
        <FileGraphCanvasInner {...props} />
      </ReactFlowProvider>
    </section>
  )
}
