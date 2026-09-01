import type { Viewport } from "@xyflow/react"
import type { GraphPosition, GraphView } from "../types/graphView"

export type ViewAction =
  | { type: "hydrate"; view: GraphView }
  | { type: "select"; nodeId: string | null }
  | { type: "search"; value: string }
  | { type: "filter"; name: "violationsOnly" | "directOnly"; value: boolean }
  | { type: "position"; nodeId: string; position: GraphPosition }
  | { type: "collapse"; nodeId: string }
  | { type: "annotate"; nodeId: string; text: string }
  | { type: "viewport"; viewport: Viewport }
  | { type: "panel"; panel: "summaryOpen" | "inspectorOpen" | "activityOpen"; value: boolean }
  | { type: "reset-layout" }

export function viewReducer(state: GraphView, action: ViewAction): GraphView {
  switch (action.type) {
    case "hydrate":
      return action.view
    case "select":
      return { ...state, selectedNodeId: action.nodeId }
    case "search":
      return { ...state, filters: { ...state.filters, search: action.value } }
    case "filter":
      return { ...state, filters: { ...state.filters, [action.name]: action.value } }
    case "position":
      return {
        ...state,
        pinnedPositions: { ...state.pinnedPositions, [action.nodeId]: action.position },
      }
    case "collapse": {
      const collapsed = new Set(state.collapsedNodeIds)
      if (collapsed.has(action.nodeId)) collapsed.delete(action.nodeId)
      else collapsed.add(action.nodeId)
      return { ...state, collapsedNodeIds: [...collapsed] }
    }
    case "annotate":
      return { ...state, annotations: { ...state.annotations, [action.nodeId]: action.text } }
    case "viewport":
      if (
        state.viewport?.x === action.viewport.x
        && state.viewport?.y === action.viewport.y
        && state.viewport?.zoom === action.viewport.zoom
      ) return state
      return { ...state, viewport: action.viewport }
    case "panel":
      return { ...state, [action.panel]: action.value }
    case "reset-layout":
      return { ...state, pinnedPositions: {}, collapsedNodeIds: [], viewport: null }
  }
}
