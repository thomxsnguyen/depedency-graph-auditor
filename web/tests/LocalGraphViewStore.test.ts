import { LocalGraphViewStore } from "../src/data/LocalGraphViewStore"
import { initialGraphView } from "../src/types/graphView"

describe("LocalGraphViewStore", () => {
  beforeEach(() => window.localStorage.clear())

  it("round-trips a saved presentation view", async () => {
    const store = new LocalGraphViewStore()
    const view = {
      ...initialGraphView,
      selectedNodeId: "vite@7.1.2",
      canvasBoxes: [
        { id: "canvas-box-1", label: "Review", position: { x: 12, y: 24 } },
        { id: "canvas-box-2", label: "Decision", position: { x: 240, y: 24 } },
      ],
      canvasArrows: [
        { id: "canvas-arrow-1", sourceBoxId: "canvas-box-1", targetBoxId: "canvas-box-2" },
      ],
    }
    await store.save("audit", view)
    await expect(store.load("audit")).resolves.toEqual(view)
  })

  it("returns null when no view has been saved", async () => {
    await expect(new LocalGraphViewStore().load("missing")).resolves.toBeNull()
  })

  it("defaults missing collections and discards invalid presentation elements", async () => {
    window.localStorage.setItem("dependency-audit-view:v4:audit", JSON.stringify({
      ...initialGraphView,
      canvasBoxes: [
        { id: "canvas-box-1", label: " Valid ", position: { x: 10, y: 20 } },
        { id: "canvas-box-bad", label: "Bad", position: { x: "no", y: 20 } },
      ],
      canvasArrows: [
        { id: "canvas-arrow-1", sourceBoxId: "canvas-box-1", targetBoxId: "missing" },
        { id: "canvas-arrow-2", sourceBoxId: "canvas-box-1", targetBoxId: "canvas-box-1" },
      ],
    }))

    const loaded = await new LocalGraphViewStore().load("audit")
    expect(loaded?.canvasBoxes).toEqual([
      { id: "canvas-box-1", label: "Valid", position: { x: 10, y: 20 } },
    ])
    expect(loaded?.canvasArrows).toEqual([])
  })

  it("returns null for malformed saved JSON", async () => {
    window.localStorage.setItem("dependency-audit-view:v4:audit", "{")
    await expect(new LocalGraphViewStore().load("audit")).resolves.toBeNull()
  })
})
