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
})
