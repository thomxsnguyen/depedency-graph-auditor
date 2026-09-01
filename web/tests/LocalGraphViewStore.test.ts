import { LocalGraphViewStore } from "../src/data/LocalGraphViewStore"
import { initialGraphView } from "../src/types/graphView"

describe("LocalGraphViewStore", () => {
  beforeEach(() => window.localStorage.clear())

  it("round-trips a saved presentation view", async () => {
    const store = new LocalGraphViewStore()
    const view = { ...initialGraphView, selectedNodeId: "vite@7.1.2" }
    await store.save("audit", view)
    await expect(store.load("audit")).resolves.toEqual(view)
  })

  it("returns null when no view has been saved", async () => {
    await expect(new LocalGraphViewStore().load("missing")).resolves.toBeNull()
  })
})
