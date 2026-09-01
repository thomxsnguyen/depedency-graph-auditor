import type { Viewport } from "@xyflow/react"

export interface GraphFilters {
  search: string
  violationsOnly: boolean
  completeGraph: boolean
}

export interface GraphPosition {
  x: number
  y: number
}

export interface CanvasBox {
  id: string
  label: string
  position: GraphPosition
}

export interface CanvasArrow {
  id: string
  sourceBoxId: string
  targetBoxId: string
}

export interface GraphView {
  selectedNodeId: string | null
  filters: GraphFilters
  pinnedPositions: Record<string, GraphPosition>
  collapsedNodeIds: string[]
  annotations: Record<string, string>
  canvasBoxes: CanvasBox[]
  canvasArrows: CanvasArrow[]
  viewport: Viewport | null
  summaryOpen: boolean
  inspectorOpen: boolean
  activityOpen: boolean
}

export const initialGraphView: GraphView = {
  selectedNodeId: null,
  filters: {
    search: "",
    violationsOnly: false,
    completeGraph: false,
  },
  pinnedPositions: {},
  collapsedNodeIds: [],
  annotations: {},
  canvasBoxes: [],
  canvasArrows: [],
  viewport: null,
  summaryOpen: false,
  inspectorOpen: false,
  activityOpen: false,
}
