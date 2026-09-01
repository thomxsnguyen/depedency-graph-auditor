import sampleAudit from "../src/data/sample-audit.json"
import { countStatuses, type AuditSnapshot } from "../src/types/audit"
import { findPath, visibleGraph } from "../src/graph/graphSelectors"

const snapshot: AuditSnapshot = {
  ...(sampleAudit as Omit<AuditSnapshot, "counts">),
  counts: countStatuses(sampleAudit.packages as AuditSnapshot["packages"]),
}

describe("graph selectors", () => {
  it("finds a deterministic shortest path through a diamond", () => {
    expect(findPath("scheduler@0.26.0", snapshot.edges)).toEqual([
      "root",
      "react-dom@19.1.0",
      "scheduler@0.26.0",
    ])
  })

  it("filters violations without changing the snapshot", () => {
    const before = structuredClone(snapshot)
    const visible = visibleGraph(
      snapshot,
      { search: "", violationsOnly: true, completeGraph: false },
      [],
    )

    expect(visible.packages.map((row) => row.id)).toEqual(["legacy-widget@1.4.0"])
    expect(visible.edges.map((edge) => edge.id)).toEqual(["root-legacy"])
    expect(snapshot).toEqual(before)
  })

  it("collapses cyclic descendants without recursing forever", () => {
    const visible = visibleGraph(
      snapshot,
      { search: "", violationsOnly: false, completeGraph: true },
      ["cycle-a@1.0.0"],
    )

    expect(visible.packages.some((row) => row.id === "cycle-a@1.0.0")).toBe(true)
    expect(visible.packages.some((row) => row.id === "cycle-b@1.0.0")).toBe(false)
  })

  it("keeps only root children in overview mode", () => {
    const visible = visibleGraph(
      snapshot,
      { search: "", violationsOnly: false, completeGraph: false },
      [],
    )

    expect(visible.packages.some((row) => row.id === "vite@7.1.2")).toBe(true)
    expect(visible.packages.some((row) => row.id === "rollup@4.50.0")).toBe(false)
  })

  it("includes transitive packages in complete graph mode", () => {
    const visible = visibleGraph(
      snapshot,
      { search: "", violationsOnly: false, completeGraph: true },
      [],
    )

    expect(visible.packages.some((row) => row.id === "rollup@4.50.0")).toBe(true)
  })
})
