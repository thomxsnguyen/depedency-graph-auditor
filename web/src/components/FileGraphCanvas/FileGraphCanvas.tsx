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
import {
  mapFileGraph,
  relationshipStrokePattern,
  type ConnectionDisplayMode,
  type FileGraphFlowNode,
} from "../../graph/mapFileGraph"
import { fileMatchesSearch } from "../../graph/fileGraphSelectors"
import { categoryDetails, classifyFile, FILE_CATEGORY_DETAILS } from "../../graph/fileCategory"
import {
  buildHierarchicalFileGraph,
  expandedArchitectureItems,
  FILE_RELATIONSHIP_DETAILS,
  type DependencyHopScope,
  type ExpandedArchitectureItem,
} from "../../graph/hierarchicalFileGraph"
import { ArchitectureNode } from "./ArchitectureNode"
import { DomainNode } from "./DomainNode"
import { FileNode } from "./FileNode"

const nodeTypes: NodeTypes = {
  architecture: ArchitectureNode,
  domain: DomainNode,
  file: FileNode,
}

interface FileGraphCanvasProps {
  snapshot: FileGraphSnapshot
  search: string
  selectedPath: string | null
  selectedGroupId: string | null
  expandedArchitectureIds: ReadonlySet<string>
  expandedDomainIds: ReadonlySet<string>
  hopScope: DependencyHopScope
  positions: Record<string, GraphPosition>
  viewport: Viewport | null
  onSelect: (path: string | null) => void
  onSelectGroup: (id: string | null) => void
  onToggleArchitecture: (id: string, project: string) => void
  onToggleDomain: (id: string, architectureId: string) => void
  onCollapseExpansion: (item: ExpandedArchitectureItem) => void
  onHopScope: (scope: DependencyHopScope) => void
  onPosition: (id: string, position: GraphPosition) => void
  onViewport: (viewport: Viewport) => void
  onReset: () => void
  onClearSearch: () => void
}

function FileCanvasControls({
  selectedPath,
  hopScope,
  connectionMode,
  onHopScope,
  onConnectionMode,
  onReset,
}: Pick<FileGraphCanvasProps, "selectedPath" | "hopScope" | "onHopScope" | "onReset"> & {
  connectionMode: ConnectionDisplayMode
  onConnectionMode: (mode: ConnectionDisplayMode) => void
}) {
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
      <span className="toolbar-divider" aria-hidden="true" />
      <div className="connection-mode" role="group" aria-label="Connection display">
        {(["focused", "all"] as const).map((mode) => (
          <button
            key={mode}
            type="button"
            aria-pressed={connectionMode === mode}
            onClick={() => onConnectionMode(mode)}
          >
            {mode === "focused" ? "Trace" : "All links"}
          </button>
        ))}
      </div>
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

function ExpandedArchitectureControls({
  items,
  onCollapse,
}: { items: readonly ExpandedArchitectureItem[]; onCollapse: (item: ExpandedArchitectureItem) => void }) {
  if (items.length === 0) return null
  return (
    <aside className="expanded-modules" aria-label="Expanded architecture">
      <span>Expanded</span>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          onClick={() => onCollapse(item)}
          aria-label={`Collapse ${item.kind} ${item.label}`}
        >
          {item.label}
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

function RelationshipLegend({ relationships }: { relationships: ReadonlySet<string> }) {
  if (relationships.size < 2) return null
  return (
    <aside className="file-relationship-legend" aria-label="Dependency relationship legend">
      <span className="file-category-legend__title">Relationships</span>
      <ul>
        {FILE_RELATIONSHIP_DETAILS.filter(({ relationship }) => relationships.has(relationship)).map(({ relationship, label, color }) => (
          <li key={relationship}>
            <svg width="20" height="8" viewBox="0 0 20 8" aria-hidden="true">
              <line
                x1="1"
                y1="4"
                x2="19"
                y2="4"
                stroke={color}
                strokeWidth="2"
                strokeDasharray={relationshipStrokePattern(relationship)}
              />
            </svg>
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
  const [connectionMode, setConnectionMode] = useState<ConnectionDisplayMode>("focused")
  const initialViewportRef = useRef(props.viewport)
  const completedInitialLayoutRef = useRef(false)
  const positionsRef = useRef(props.positions)

  useEffect(() => {
    positionsRef.current = props.positions
  }, [props.positions])

  const visibleGraph = useMemo(
    () => buildHierarchicalFileGraph(
      props.snapshot,
      props.expandedArchitectureIds,
      props.expandedDomainIds,
      props.selectedPath,
      props.hopScope,
      props.search,
    ),
    [props.expandedArchitectureIds, props.expandedDomainIds, props.hopScope, props.search, props.selectedPath, props.snapshot],
  )
  const expansionItems = useMemo(
    () => expandedArchitectureItems(props.snapshot, props.expandedArchitectureIds, props.expandedDomainIds),
    [props.expandedArchitectureIds, props.expandedDomainIds, props.snapshot],
  )
  const mapped = useMemo(
    () => mapFileGraph(
      visibleGraph,
      props.selectedPath,
      props.selectedGroupId,
      props.search,
      props.positions,
      connectionMode,
    ),
    [connectionMode, props.positions, props.search, props.selectedGroupId, props.selectedPath, visibleGraph],
  )
  const relationships = useMemo(
    () => new Set(mapped.edges.map((edge) => {
      const match = /file-edge--([^\s]+)/.exec(edge.className ?? "")
      return match?.[1] ?? ""
    }).filter(Boolean)),
    [mapped.edges],
  )
  const interactiveNodes = useMemo(
    () => mapped.nodes.map((node): FileGraphFlowNode => {
      if (node.type === "architecture") {
        return {
          ...node,
          data: { ...node.data, onSelect: props.onSelectGroup, onToggle: props.onToggleArchitecture },
        }
      }
      if (node.type === "domain") {
        return {
          ...node,
          data: { ...node.data, onSelect: props.onSelectGroup, onToggle: props.onToggleDomain },
        }
      }
      return { ...node, data: { ...node.data, onSelect: props.onSelect } }
    }),
    [mapped.nodes, props.onSelect, props.onSelectGroup, props.onToggleArchitecture, props.onToggleDomain],
  )
  const [nodes, setNodes, onNodesChange] = useNodesState<FileGraphFlowNode>(interactiveNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(mapped.edges)

  useEffect(() => {
    setEdges(mapped.edges)
    setNodes((currentNodes) => {
      const current = new Map(currentNodes.map((node) => [node.id, node]))
      return interactiveNodes.map((node) => ({
        ...node,
        position: node.parentId
          ? node.position
          : props.positions[node.id] ?? current.get(node.id)?.position ?? node.position,
        selected: node.type === "file" ? node.data.path === props.selectedPath : node.id === props.selectedGroupId,
      }))
    })
  }, [interactiveNodes, mapped.edges, props.positions, props.selectedGroupId, props.selectedPath, setEdges, setNodes])

  const layoutNodes = useMemo(
    () => mapped.nodes.filter((node) => node.type === "architecture").map((node) => ({
      id: node.id,
      width: typeof node.style?.width === "number" ? node.style.width : 260,
      height: typeof node.style?.height === "number" ? node.style.height : 96,
      rank: node.data.rank,
      lane: node.data.lane,
    })),
    [mapped.nodes],
  )
  const layoutEdges = useMemo(
    () => {
      const architectureIds = new Set(layoutNodes.map((node) => node.id))
      return mapped.edges
        .filter((edge) => architectureIds.has(edge.source) && architectureIds.has(edge.target))
        .map((edge) => ({ id: edge.id, source: edge.source, target: edge.target }))
    },
    [layoutNodes, mapped.edges],
  )

  useEffect(() => {
    let active = true
    let fitFrame: number | null = null
    const nestedPositions = new Map(interactiveNodes.map((node) => [node.id, node.position]))
    layoutGraph(layoutNodes, layoutEdges)
      .then((layoutPositions) => {
        if (!active) return
        setNodes((currentNodes) => currentNodes.map((node) => ({
          ...node,
          position: node.parentId
            ? nestedPositions.get(node.id) ?? node.position
            : positionsRef.current[node.id] ?? layoutPositions[node.id] ?? node.position,
        })))
        const shouldFit = completedInitialLayoutRef.current || initialViewportRef.current === null
        completedInitialLayoutRef.current = true
        if (shouldFit) fitFrame = window.requestAnimationFrame(() => flow.fitView({ padding: 0.2, duration: 220 }))
      })
      .catch(() => {
        // Deterministic fallback positions remain usable when layout fails.
      })
    return () => {
      active = false
      if (fitFrame !== null) window.cancelAnimationFrame(fitFrame)
    }
  }, [flow, interactiveNodes, layoutEdges, layoutNodes, layoutRequest, setNodes])

  const resetLayout = useCallback(() => {
    positionsRef.current = {}
    onReset()
    setLayoutRequest((request) => request + 1)
  }, [onReset])

  const clearSelection = () => {
    props.onSelect(null)
    props.onSelectGroup(null)
  }

  const changeConnectionMode = (mode: ConnectionDisplayMode) => {
    setConnectionMode(mode)
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={(_, node) => {
        if (node.type === "file") props.onSelect(node.data.path)
        else props.onSelectGroup(node.id)
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
        connectionMode={connectionMode}
        onHopScope={props.onHopScope}
        onConnectionMode={changeConnectionMode}
        onReset={resetLayout}
      />
      {connectionMode === "focused" && expansionItems.length === 0 && props.selectedPath === null && props.selectedGroupId === null && (
        <aside className="connection-hint" aria-label="Connection guidance">
          <strong>Flow runs left to right</strong>
          <span>Select a group to trace only its incoming and outgoing links. Click the canvas to clear.</span>
        </aside>
      )}
      <ExpandedArchitectureControls items={expansionItems} onCollapse={props.onCollapseExpansion} />
      <FileCategoryLegend paths={props.snapshot.nodes.map((node) => node.path)} />
      <RelationshipLegend relationships={relationships} />
      <MiniMap
        className="graph-minimap"
        pannable
        zoomable
        nodeColor={(node) => {
          if (node.data.entityKind === "architecture") return "#51555b"
          if (node.data.entityKind === "domain") return "#73777d"
          return categoryDetails(classifyFile(typeof node.data.path === "string" ? node.data.path : "")).color
        }}
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
