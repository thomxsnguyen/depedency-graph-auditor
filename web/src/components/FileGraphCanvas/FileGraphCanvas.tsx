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
import { fileEdgeId, fileNodeId, mapFileGraph, type FileGraphFlowNode } from "../../graph/mapFileGraph"
import { fileMatchesSearch } from "../../graph/fileGraphSelectors"
import { FileNode } from "./FileNode"

const nodeTypes: NodeTypes = { file: FileNode }

interface FileGraphCanvasProps {
  snapshot: FileGraphSnapshot
  search: string
  selectedPath: string | null
  positions: Record<string, GraphPosition>
  viewport: Viewport | null
  onSelect: (path: string | null) => void
  onPosition: (path: string, position: GraphPosition) => void
  onViewport: (viewport: Viewport) => void
  onReset: () => void
  onClearSearch: () => void
}

function FileCanvasControls({ onReset }: { onReset: () => void }) {
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
    </div>
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

  const mapped = useMemo(
    () => mapFileGraph(props.snapshot, props.selectedPath, props.search, props.positions),
    [props.positions, props.search, props.selectedPath, props.snapshot],
  )
  const interactiveNodes = useMemo(
    () => mapped.nodes.map((node) => ({
      ...node,
      data: { ...node.data, onSelect: props.onSelect },
    })),
    [mapped.nodes, props.onSelect],
  )
  const [nodes, setNodes, onNodesChange] = useNodesState<FileGraphFlowNode>(interactiveNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(mapped.edges)

  useEffect(() => {
    setEdges(mapped.edges)
    setNodes((currentNodes) => {
      const current = new Map(currentNodes.map((node) => [node.id, node]))
      return interactiveNodes.map((node) => ({
        ...node,
        position: props.positions[node.data.path] ?? current.get(node.id)?.position ?? node.position,
        selected: node.data.path === props.selectedPath,
      }))
    })
  }, [interactiveNodes, mapped.edges, props.positions, props.selectedPath, setEdges, setNodes])

  const layoutNodes = useMemo(
    () => props.snapshot.nodes.map((node) => ({ id: fileNodeId(node.path) })),
    [props.snapshot.nodes],
  )
  const layoutEdges = useMemo(
    () => props.snapshot.edges.map((edge) => ({
      id: fileEdgeId(edge.from, edge.to),
      source: fileNodeId(edge.from),
      target: fileNodeId(edge.to),
    })),
    [props.snapshot.edges],
  )

  useEffect(() => {
    let active = true
    let fitFrame: number | null = null
    layoutGraph(layoutNodes, layoutEdges)
      .then((layoutPositions) => {
        if (!active) return
        setNodes((currentNodes) => currentNodes.map((node) => ({
          ...node,
          position: positionsRef.current[node.data.path] ?? layoutPositions[node.id] ?? node.position,
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

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={(_, node) => props.onSelect(node.data.path)}
      onPaneClick={() => props.onSelect(null)}
      onNodeDragStop={(_, node) => props.onPosition(node.data.path, node.position)}
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
      <FileCanvasControls onReset={resetLayout} />
      <MiniMap
        className="graph-minimap"
        pannable
        zoomable
        nodeColor="#ffffff"
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
