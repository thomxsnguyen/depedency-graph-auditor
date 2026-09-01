import { createHistory, historyReducer } from "../src/state/historyReducer"
import { viewReducer, type ViewAction } from "../src/state/viewReducer"
import { initialGraphView, type GraphView } from "../src/types/graphView"

describe("presentation state", () => {
  it("keeps prior view state immutable", () => {
    const before = structuredClone(initialGraphView)
    const next = viewReducer(initialGraphView, {
      type: "position",
      nodeId: "react@19.1.0",
      position: { x: 10, y: 20 },
    })

    expect(next.pinnedPositions["react@19.1.0"]).toEqual({ x: 10, y: 20 })
    expect(initialGraphView).toEqual(before)
  })

  it("supports bounded undo and redo", () => {
    const reduceHistory = historyReducer<GraphView, ViewAction>(viewReducer, 2)
    let state = createHistory(initialGraphView)
    state = reduceHistory(state, { type: "apply", action: { type: "search", value: "react" } })
    state = reduceHistory(state, { type: "apply", action: { type: "search", value: "vite" } })
    state = reduceHistory(state, { type: "undo" })
    expect(state.present.filters.search).toBe("react")
    state = reduceHistory(state, { type: "redo" })
    expect(state.present.filters.search).toBe("vite")
  })

  it("resets layout without resetting selection and filters", () => {
    const prepared = {
      ...initialGraphView,
      selectedNodeId: "vite@7.1.2",
      filters: { ...initialGraphView.filters, search: "vite" },
      pinnedPositions: { "vite@7.1.2": { x: 10, y: 20 } },
      collapsedNodeIds: ["vite@7.1.2"],
    }
    const next = viewReducer(prepared, { type: "reset-layout" })

    expect(next.selectedNodeId).toBe("vite@7.1.2")
    expect(next.filters.search).toBe("vite")
    expect(next.pinnedPositions).toEqual({})
    expect(next.collapsedNodeIds).toEqual([])
  })

  it("adds, renames, and moves a presentation box immutably", () => {
    const box = { id: "canvas-box-1", label: "Untitled", position: { x: 10, y: 20 } }
    const added = viewReducer(initialGraphView, { type: "box.add", box })
    const renamed = viewReducer(added, { type: "box.rename", boxId: box.id, label: "  Review path  " })
    const moved = viewReducer(renamed, { type: "box.move", boxId: box.id, position: { x: 30, y: 40 } })

    expect(initialGraphView.canvasBoxes).toEqual([])
    expect(moved.canvasBoxes).toEqual([
      { id: box.id, label: "Review path", position: { x: 30, y: 40 } },
    ])
  })

  it("validates arrows and deletes attached arrows with a box", () => {
    const first = { id: "canvas-box-1", label: "First", position: { x: 0, y: 0 } }
    const second = { id: "canvas-box-2", label: "Second", position: { x: 200, y: 0 } }
    let state = viewReducer(initialGraphView, { type: "box.add", box: first })
    state = viewReducer(state, { type: "box.add", box: second })
    state = viewReducer(state, {
      type: "arrow.add",
      arrow: { id: "canvas-arrow-1", sourceBoxId: first.id, targetBoxId: second.id },
    })

    const duplicate = viewReducer(state, {
      type: "arrow.add",
      arrow: { id: "canvas-arrow-2", sourceBoxId: first.id, targetBoxId: second.id },
    })
    const selfArrow = viewReducer(state, {
      type: "arrow.add",
      arrow: { id: "canvas-arrow-3", sourceBoxId: first.id, targetBoxId: first.id },
    })
    const deleted = viewReducer(state, { type: "box.delete", boxId: first.id })

    expect(state.canvasArrows).toHaveLength(1)
    expect(duplicate).toBe(state)
    expect(selfArrow).toBe(state)
    expect(deleted.canvasBoxes).toEqual([second])
    expect(deleted.canvasArrows).toEqual([])
  })

  it("preserves presentation elements when resetting audit layout", () => {
    const prepared = {
      ...initialGraphView,
      canvasBoxes: [{ id: "canvas-box-1", label: "Note", position: { x: 1, y: 2 } }],
      canvasArrows: [],
      pinnedPositions: { root: { x: 5, y: 6 } },
    }

    const next = viewReducer(prepared, { type: "reset-layout" })
    expect(next.canvasBoxes).toEqual(prepared.canvasBoxes)
    expect(next.canvasArrows).toEqual([])
    expect(next.pinnedPositions).toEqual({})
  })
})
