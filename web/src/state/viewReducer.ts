import type { Viewport } from "@xyflow/react"
import type { CanvasArrow, CanvasBox, GraphPosition, GraphView } from "../types/graphView"

export type ViewAction =
  | { type: "hydrate"; view: GraphView }
  | { type: "select"; nodeId: string | null }
  | { type: "search"; value: string }
  | { type: "filter"; name: "violationsOnly" | "completeGraph"; value: boolean }
  | { type: "position"; nodeId: string; position: GraphPosition }
  | { type: "collapse"; nodeId: string }
  | { type: "annotate"; nodeId: string; text: string }
  | { type: "box.add"; box: CanvasBox }
  | { type: "box.rename"; boxId: string; label: string }
  | { type: "box.move"; boxId: string; position: GraphPosition }
  | { type: "box.delete"; boxId: string }
  | { type: "arrow.add"; arrow: CanvasArrow }
  | { type: "arrow.delete"; arrowId: string }
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
    case "box.add":
      if (state.canvasBoxes.some((box) => box.id === action.box.id)) return state
      return { ...state, canvasBoxes: [...state.canvasBoxes, action.box] }
    case "box.rename": {
      const label = action.label.trim() || "Untitled"
      const current = state.canvasBoxes.find((box) => box.id === action.boxId)
      if (!current || current.label === label) return state
      return {
        ...state,
        canvasBoxes: state.canvasBoxes.map((box) => box.id === action.boxId ? { ...box, label } : box),
      }
    }
    case "box.move": {
      const current = state.canvasBoxes.find((box) => box.id === action.boxId)
      if (
        !current
        || (current.position.x === action.position.x && current.position.y === action.position.y)
      ) return state
      return {
        ...state,
        canvasBoxes: state.canvasBoxes.map((box) => box.id === action.boxId
          ? { ...box, position: action.position }
          : box),
      }
    }
    case "box.delete":
      if (!state.canvasBoxes.some((box) => box.id === action.boxId)) return state
      return {
        ...state,
        canvasBoxes: state.canvasBoxes.filter((box) => box.id !== action.boxId),
        canvasArrows: state.canvasArrows.filter(
          (arrow) => arrow.sourceBoxId !== action.boxId && arrow.targetBoxId !== action.boxId,
        ),
      }
    case "arrow.add": {
      const { arrow } = action
      const boxIds = new Set(state.canvasBoxes.map((box) => box.id))
      if (
        state.canvasArrows.some((existing) => existing.id === arrow.id)
        || !boxIds.has(arrow.sourceBoxId)
        || !boxIds.has(arrow.targetBoxId)
        || arrow.sourceBoxId === arrow.targetBoxId
        || state.canvasArrows.some(
          (existing) => existing.sourceBoxId === arrow.sourceBoxId
            && existing.targetBoxId === arrow.targetBoxId,
        )
      ) return state
      return { ...state, canvasArrows: [...state.canvasArrows, arrow] }
    }
    case "arrow.delete":
      if (!state.canvasArrows.some((arrow) => arrow.id === action.arrowId)) return state
      return {
        ...state,
        canvasArrows: state.canvasArrows.filter((arrow) => arrow.id !== action.arrowId),
      }
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
