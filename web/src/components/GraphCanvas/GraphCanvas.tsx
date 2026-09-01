import { useEffect, useMemo } from "react"
import { Focus, Redo2, RotateCcw, Undo2, ZoomIn, ZoomOut } from "lucide-react"
import {
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type Edge,
  type NodeTypes,
  type Viewport,
} from "@xyflow/react"
import type { AuditSnapshot } from "../../types/audit"
import type { GraphPosition, GraphView } from "../../types/graphView"
import { visibleGraph } from "../../graph/graphSelectors"
import { layoutGraph } from "../../graph/layoutGraph"
import { mapAuditGraph, type AuditGraphNode } from "../../graph/mapAuditGraph"
import { PackageNode } from "./PackageNode"

const nodeTypes: NodeTypes = { auditPackage: PackageNode }

interface GraphCanvasProps {
  snapshot: AuditSnapshot
  view: GraphView
  canUndo: boolean
  canRedo: boolean
  onSelect: (nodeId: string) => void
  onCollapse: (nodeId: string) => void
  onPosition: (nodeId: string, position: GraphPosition) => void
  onViewport: (viewport: Viewport) => void
  onUndo: () => void
  onRedo: () => void
  onResetLayout: () => void
}

function CanvasControls({ canUndo, canRedo, onUndo, onRedo, onResetLayout }: Pick<GraphCanvasProps, "canUndo" | "canRedo" | "onUndo" | "onRedo" | "onResetLayout">) {
  const flow = useReactFlow()

  return (
    <div className="graph-toolbar" aria-label="Graph controls">
      <button className="icon-button" type="button" onClick={() => flow.zoomOut()} aria-label="Zoom out">
        <ZoomOut size={17} aria-hidden="true" />
      </button>
      <button className="icon-button" type="button" onClick={() => flow.fitView({ padding: 0.2, duration: 220 })} aria-label="Fit graph to view">
        <Focus size={17} aria-hidden="true" />
      </button>
      <button className="icon-button" type="button" onClick={() => flow.zoomIn()} aria-label="Zoom in">
        <ZoomIn size={17} aria-hidden="true" />
      </button>
      <span className="toolbar-divider" aria-hidden="true" />
      <button className="icon-button" type="button" onClick={onUndo} disabled={!canUndo} aria-label="Undo presentation change">
        <Undo2 size={17} aria-hidden="true" />
      </button>
      <button className="icon-button" type="button" onClick={onRedo} disabled={!canRedo} aria-label="Redo presentation change">
        <Redo2 size={17} aria-hidden="true" />
      </button>
      <button className="icon-button" type="button" onClick={onResetLayout} aria-label="Reset graph layout">
        <RotateCcw size={17} aria-hidden="true" />
      </button>
    </div>
  )
}

function GraphCanvasInner(props: GraphCanvasProps) {
  const flow = useReactFlow()
  const visible = useMemo(
    () => visibleGraph(props.snapshot, props.view.filters, props.view.collapsedNodeIds),
    [props.snapshot, props.view.filters, props.view.collapsedNodeIds],
  )
  const mapped = useMemo(
    () => mapAuditGraph(
      props.snapshot,
      visible.packages,
      visible.edges,
      props.view.selectedNodeId,
      props.view.collapsedNodeIds,
      props.view.annotations,
      props.view.pinnedPositions,
    ),
    [
      props.snapshot,
      visible,
      props.view.selectedNodeId,
      props.view.collapsedNodeIds,
      props.view.annotations,
      props.view.pinnedPositions,
    ],
  )
  const interactiveNodes = useMemo(
    () => mapped.nodes.map((node) => ({
      ...node,
      data: { ...node.data, onSelect: props.onSelect, onCollapse: props.onCollapse },
    })),
    [mapped.nodes, props.onSelect, props.onCollapse],
  )
  const [nodes, setNodes, onNodesChange] = useNodesState<AuditGraphNode>(interactiveNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>(mapped.edges)

  useEffect(() => {
    let active = true
    let fitFrame: number | null = null
    const shouldFitAfterLayout = props.view.viewport === null
    setEdges(mapped.edges)
    setNodes(interactiveNodes)

    layoutGraph(interactiveNodes, mapped.edges)
      .then((positions) => {
        if (!active) return
        const positionedNodes = interactiveNodes.map((node) => ({
          ...node,
          position: props.view.pinnedPositions[node.id] ?? positions[node.id] ?? node.position,
        }))
        setNodes(positionedNodes)

        if (shouldFitAfterLayout) {
          fitFrame = window.requestAnimationFrame(() => {
            flow.fitView({ nodes: positionedNodes, padding: 0.2, duration: 220 })
          })
        }
      })
      .catch(() => {
        // The deterministic fallback positions remain usable if the worker fails.
      })

    return () => {
      active = false
      if (fitFrame !== null) window.cancelAnimationFrame(fitFrame)
    }
  }, [flow, interactiveNodes, mapped.edges, props.view.pinnedPositions, props.view.viewport, setEdges, setNodes])

  const hasActiveFilter = Boolean(
    props.view.filters.search.trim()
    || props.view.filters.directOnly
    || props.view.filters.violationsOnly,
  )

  if (visible.packages.length === 0 && hasActiveFilter) {
    return (
      <div className="graph-empty" role="status">
        <strong>No packages match these filters.</strong>
        <span>Clear search or filter selections to restore the graph.</span>
      </div>
    )
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={(_, node) => props.onSelect(node.id)}
      onNodeDragStop={(_, node) => props.onPosition(node.id, node.position)}
      onMoveEnd={(_, viewport) => props.onViewport(viewport)}
      defaultViewport={props.view.viewport ?? { x: 0, y: 0, zoom: 1 }}
      fitView={!props.view.viewport}
      fitViewOptions={{ padding: 0.2 }}
      minZoom={0.25}
      maxZoom={1.8}
      nodesConnectable={false}
      deleteKeyCode={null}
      selectionOnDrag={false}
      onlyRenderVisibleElements
      aria-label="Interactive dependency graph"
    >
      <CanvasControls
        canUndo={props.canUndo}
        canRedo={props.canRedo}
        onUndo={props.onUndo}
        onRedo={props.onRedo}
        onResetLayout={props.onResetLayout}
      />
      <MiniMap
        className="graph-minimap"
        pannable
        zoomable
        nodeColor={(node) => node.id === "root" ? "#1d1d1f" : "#ffffff"}
        nodeStrokeColor="#d2d2d7"
        maskColor="rgba(245,245,247,0.78)"
        ariaLabel="Dependency graph minimap"
      />
    </ReactFlow>
  )
}

export function GraphCanvas(props: GraphCanvasProps) {
  return (
    <section className="graph-canvas" aria-label="Dependency graph canvas">
      <ReactFlowProvider>
        <GraphCanvasInner {...props} />
      </ReactFlowProvider>
    </section>
  )
}
