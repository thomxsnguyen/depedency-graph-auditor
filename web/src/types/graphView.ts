import type { Viewport } from "@xyflow/react"

export interface GraphFilters {
  search: string
  violationsOnly: boolean
  directOnly: boolean
}

export interface GraphPosition {
  x: number
  y: number
}

export interface GraphView {
  selectedNodeId: string | null
  filters: GraphFilters
  pinnedPositions: Record<string, GraphPosition>
  collapsedNodeIds: string[]
  annotations: Record<string, string>
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
    directOnly: false,
  },
  pinnedPositions: {},
  collapsedNodeIds: [],
  annotations: {},
  viewport: null,
  summaryOpen: false,
  inspectorOpen: false,
  activityOpen: false,
}
