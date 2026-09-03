import { FixtureFileGraphDataSource, parseFileGraph } from "../src/data/FixtureFileGraphDataSource"

const validGraph = {
  schemaVersion: 1,
  root: "demo",
  nodes: [{ path: "src/main.ts" }, { path: "src/lib.ts" }],
  edges: [{ from: "src/main.ts", to: "src/lib.ts" }],
  diagnostics: [{ path: "src/main.ts", import: "./missing", message: "unresolved local import" }],
}

describe("file graph data source", () => {
  it("loads the deterministic language-neutral fixture", async () => {
    const graph = await new FixtureFileGraphDataSource().load()
    expect(graph.schemaVersion).toBe(1)
    expect(graph.nodes.some((node) => node.path.endsWith(".go"))).toBe(true)
    expect(graph.nodes.some((node) => node.path.endsWith(".py"))).toBe(true)
    expect(graph.nodes.some((node) => node.path.endsWith(".tsx"))).toBe(true)
  })

  it("accepts a valid normalized graph", () => {
    expect(parseFileGraph(validGraph)).toEqual(validGraph)
  })

  it("rejects unsupported versions, duplicate nodes, and unknown edge endpoints", () => {
    expect(() => parseFileGraph({ ...validGraph, schemaVersion: 2 })).toThrow()
    expect(() => parseFileGraph({
      ...validGraph,
      nodes: [{ path: "src/main.ts" }, { path: "src/main.ts" }],
      edges: [],
    })).toThrow()
    expect(() => parseFileGraph({
      ...validGraph,
      edges: [{ from: "src/main.ts", to: "src/missing.ts" }],
    })).toThrow()
  })
})
