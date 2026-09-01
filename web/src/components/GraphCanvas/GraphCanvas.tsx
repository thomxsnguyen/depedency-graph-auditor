import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Focus, Redo2, RotateCcw, SquarePlus, Undo2, ZoomIn, ZoomOut } from "lucide-react"
import {
  ConnectionLineType,
  MarkerType,
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
import type { CanvasBox, GraphPosition, GraphView } from "../../types/graphView"
import type { ViewAction } from "../../state/viewReducer"
import { visibleGraph } from "../../graph/graphSelectors"
import { layoutGraph } from "../../graph/layoutGraph"
import { mapAuditGraph, type AuditGraphNode } from "../../graph/mapAuditGraph"
import { CanvasBoxNode, type CanvasBoxGraphNode } from "./CanvasBoxNode"
import { PackageNode } from "./PackageNode"

type GraphNode = AuditGraphNode | CanvasBoxGraphNode

const nodeTypes: NodeTypes = {
  auditPackage: PackageNode,
  canvasBox: CanvasBoxNode,
}

interface GraphCanvasProps {
  snapshot: AuditSnapshot
  view: GraphView
  canUndo: boolean
  canRedo: boolean
  onSelect: (nodeId: string) => void
  onCollapse: (nodeId: string) => void
  onPosition: (nodeId: string, position: GraphPosition) => void
  onViewport: (viewport: Viewport) => void
  onViewAction: (action: ViewAction) => void
  onUndo: () => void
  onRedo: () => void
  onResetLayout: () => void
}

interface CanvasControlsProps {
  addingBox: boolean
  canUndo: boolean
  canRedo: boolean
  onAddBox: () => void
  onUndo: () => void
  onRedo: () => void
  onResetLayout: () => void
}

function CanvasControls({
  addingBox,
  canUndo,
  canRedo,
  onAddBox,
  onUndo,
  onRedo,
  onResetLayout,
}: CanvasControlsProps) {
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
      <button
        className="icon-button"
        type="button"
        onClick={onAddBox}
        aria-label={addingBox ? "Cancel adding box" : "Add presentation box"}
        aria-pressed={addingBox}
      >
        <SquarePlus size={17} aria-hidden="true" />
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

function createPresentationId(prefix: "canvas-box" | "canvas-arrow"): string {
  return `${prefix}-${crypto.randomUUID()}`
}

function GraphCanvasInner(props: GraphCanvasProps) {
  const flow = useReactFlow()
  const onViewAction = props.onViewAction
  const [layoutRequest, setLayoutRequest] = useState(0)
  const [placingBox, setPlacingBox] = useState(false)
  const [editingBoxId, setEditingBoxId] = useState<string | null>(null)
  const initialViewportRef = useRef(props.view.viewport)
  const completedInitialLayoutRef = useRef(false)
  const pinnedPositionsRef = useRef(props.view.pinnedPositions)

  useEffect(() => {
    pinnedPositionsRef.current = props.view.pinnedPositions
  }, [props.view.pinnedPositions])

  useEffect(() => {
    if (!placingBox) return
    const cancelPlacement = (event: KeyboardEvent) => {
      if (event.key === "Escape") setPlacingBox(false)
    }
    window.addEventListener("keydown", cancelPlacement)
    return () => window.removeEventListener("keydown", cancelPlacement)
  }, [placingBox])

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
  const interactiveAuditNodes = useMemo(
    () => mapped.nodes.map((node) => ({
      ...node,
      data: { ...node.data, onSelect: props.onSelect, onCollapse: props.onCollapse },
    })),
    [mapped.nodes, props.onSelect, props.onCollapse],
  )

  const beginBoxEdit = useCallback((boxId: string) => {
    setEditingBoxId(boxId)
  }, [])
  const commitBoxEdit = useCallback((boxId: string, label: string) => {
    onViewAction({ type: "box.rename", boxId, label })
    setEditingBoxId(null)
  }, [onViewAction])
  const cancelBoxEdit = useCallback(() => {
    setEditingBoxId(null)
  }, [])

  const canvasBoxNodes = useMemo<CanvasBoxGraphNode[]>(
    () => props.view.canvasBoxes.map((box) => ({
      id: box.id,
      type: "canvasBox",
      position: box.position,
      data: {
        label: box.label,
        editing: editingBoxId === box.id,
        onBeginEdit: beginBoxEdit,
        onCommitEdit: commitBoxEdit,
        onCancelEdit: cancelBoxEdit,
      },
      draggable: editingBoxId !== box.id,
      selectable: true,
      focusable: true,
      connectable: true,
      deletable: true,
      selected: editingBoxId === box.id,
      ariaLabel: `${box.label}, presentation box`,
    })),
    [
      beginBoxEdit,
      cancelBoxEdit,
      commitBoxEdit,
      editingBoxId,
      props.view.canvasBoxes,
    ],
  )

  const canvasArrowEdges = useMemo<Edge[]>(
    () => props.view.canvasArrows.map((arrow) => ({
      id: arrow.id,
      source: arrow.sourceBoxId,
      target: arrow.targetBoxId,
      type: "smoothstep",
      markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 },
      className: "canvas-arrow",
      data: { canvasArrow: true },
      selectable: true,
      focusable: true,
      deletable: true,
    })),
    [props.view.canvasArrows],
  )
  const graphNodes = useMemo<GraphNode[]>(
    () => [...interactiveAuditNodes, ...canvasBoxNodes],
    [interactiveAuditNodes, canvasBoxNodes],
  )
  const graphEdges = useMemo<Edge[]>(
    () => [...mapped.edges, ...canvasArrowEdges],
    [canvasArrowEdges, mapped.edges],
  )

  const [nodes, setNodes, onNodesChange] = useNodesState<GraphNode>(graphNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>(graphEdges)
  const layoutNodes = useMemo(
    () => [{ id: "root" }, ...visible.packages.map((packageRow) => ({ id: packageRow.id }))],
    [visible.packages],
  )
  const layoutEdges = useMemo(
    () => visible.edges.map((edge) => ({ id: edge.id, source: edge.from, target: edge.to })),
    [visible.edges],
  )

  useEffect(() => {
    setEdges((currentEdges) => {
      const selected = new Map(currentEdges.map((edge) => [edge.id, edge.selected]))
      return graphEdges.map((edge) => ({ ...edge, selected: selected.get(edge.id) ?? edge.selected }))
    })
    setNodes((currentNodes) => {
      const current = new Map(currentNodes.map((node) => [node.id, node]))
      return graphNodes.map((node) => {
        const existing = current.get(node.id)
        const position = node.type === "canvasBox"
          ? node.position
          : props.view.pinnedPositions[node.id] ?? existing?.position ?? node.position
        return {
          ...node,
          position,
          selected: node.selected || existing?.selected,
        }
      })
    })
  }, [graphEdges, graphNodes, props.view.pinnedPositions, setEdges, setNodes])

  useEffect(() => {
    let active = true
    let fitFrame: number | null = null

    layoutGraph(layoutNodes, layoutEdges)
      .then((positions) => {
        if (!active) return
        setNodes((currentNodes) => currentNodes.map((node) => ({
          ...node,
          position: node.type === "canvasBox"
            ? node.position
            : pinnedPositionsRef.current[node.id] ?? positions[node.id] ?? node.position,
        })))
        const shouldFit = completedInitialLayoutRef.current || initialViewportRef.current === null
        completedInitialLayoutRef.current = true
        if (shouldFit) {
          fitFrame = window.requestAnimationFrame(() => {
            flow.fitView({ padding: 0.2, duration: 220 })
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
  }, [flow, layoutEdges, layoutNodes, layoutRequest, setNodes])

  const canvasBoxIds = useMemo(
    () => new Set(props.view.canvasBoxes.map((box) => box.id)),
    [props.view.canvasBoxes],
  )
  const isValidCanvasConnection = useCallback((source: string | null, target: string | null) => {
    if (!source || !target || source === target) return false
    if (!canvasBoxIds.has(source) || !canvasBoxIds.has(target)) return false
    return !props.view.canvasArrows.some(
      (arrow) => arrow.sourceBoxId === source && arrow.targetBoxId === target,
    )
  }, [canvasBoxIds, props.view.canvasArrows])

  const hasActiveFilter = Boolean(
    props.view.filters.search.trim()
    || props.view.filters.completeGraph
    || props.view.filters.violationsOnly,
  )

  if (visible.packages.length === 0 && hasActiveFilter && props.view.canvasBoxes.length === 0) {
    return (
      <div className="graph-empty" role="status">
        <strong>No packages match these filters.</strong>
        <span>Clear search or filter selections to restore the graph.</span>
      </div>
    )
  }

  return (
    <ReactFlow
      className={placingBox ? "graph-flow--placing" : ""}
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={(_, node) => {
        if (node.type === "auditPackage") props.onSelect(node.id)
      }}
      onNodeDragStop={(_, node) => {
        if (node.type === "canvasBox") {
          props.onViewAction({ type: "box.move", boxId: node.id, position: node.position })
        } else {
          props.onPosition(node.id, node.position)
        }
      }}
      onNodesDelete={(deletedNodes) => {
        for (const node of deletedNodes) {
          if (node.type === "canvasBox") {
            props.onViewAction({ type: "box.delete", boxId: node.id })
            if (editingBoxId === node.id) setEditingBoxId(null)
          }
        }
      }}
      onEdgesDelete={(deletedEdges) => {
        for (const edge of deletedEdges) {
          if (edge.data?.canvasArrow === true) {
            props.onViewAction({ type: "arrow.delete", arrowId: edge.id })
          }
        }
      }}
      onConnect={(connection) => {
        if (!isValidCanvasConnection(connection.source, connection.target)) return
        props.onViewAction({
          type: "arrow.add",
          arrow: {
            id: createPresentationId("canvas-arrow"),
            sourceBoxId: connection.source!,
            targetBoxId: connection.target!,
          },
        })
      }}
      isValidConnection={(connection) => isValidCanvasConnection(connection.source, connection.target)}
      onPaneClick={(event) => {
        if (!placingBox) return
        const graphPosition = flow.screenToFlowPosition({ x: event.clientX, y: event.clientY })
        const box: CanvasBox = {
          id: createPresentationId("canvas-box"),
          label: "Untitled",
          position: { x: graphPosition.x - 90, y: graphPosition.y - 32 },
        }
        props.onViewAction({ type: "box.add", box })
        setEditingBoxId(box.id)
        setPlacingBox(false)
      }}
      onMoveEnd={(_, viewport) => props.onViewport(viewport)}
      defaultViewport={props.view.viewport ?? { x: 0, y: 0, zoom: 1 }}
      minZoom={nodes.length > 80 ? 0.05 : 0.25}
      maxZoom={1.8}
      nodesConnectable
      deleteKeyCode={["Backspace", "Delete"]}
      selectionOnDrag={false}
      panOnDrag={!placingBox}
      connectionLineType={ConnectionLineType.SmoothStep}
      aria-label="Interactive dependency graph"
    >
      <CanvasControls
        addingBox={placingBox}
        canUndo={props.canUndo}
        canRedo={props.canRedo}
        onAddBox={() => setPlacingBox((active) => !active)}
        onUndo={props.onUndo}
        onRedo={props.onRedo}
        onResetLayout={() => {
          props.onResetLayout()
          setLayoutRequest((request) => request + 1)
        }}
      />
      {placingBox && (
        <div className="placement-hint" role="status">
          Click empty canvas to place a box · Escape to cancel
        </div>
      )}
      <MiniMap
        className="graph-minimap"
        pannable
        zoomable
        nodeColor={(node) => node.id === "root" ? "#1d1d1f" : node.type === "canvasBox" ? "#fafafc" : "#ffffff"}
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
