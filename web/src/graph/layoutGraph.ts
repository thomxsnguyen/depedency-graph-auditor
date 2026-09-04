import ELK from "elkjs/lib/elk-api.js"
import ElkWorker from "elkjs/lib/elk-worker.min.js?worker"
import type { GraphPosition } from "../types/graphView"

interface LayoutNode {
  id: string
  width?: number
  height?: number
  rank?: number
  lane?: "main" | "configuration" | "shared" | "test" | "tooling"
}

const elk = new ELK({
  workerFactory: () => new ElkWorker(),
})

export function layoutGraph(
  nodes: readonly LayoutNode[],
  edges: readonly { id: string; source: string; target: string }[],
): Promise<Record<string, GraphPosition>> {
  if (nodes.length === 0) return Promise.resolve({})

  if (nodes.some((node) => node.rank !== undefined || node.lane !== undefined)) {
    const laneOrder: Record<NonNullable<LayoutNode["lane"]>, number> = {
      configuration: 0,
      main: 1,
      shared: 2,
      test: 3,
      tooling: 4,
    }
    const laneBase: Record<NonNullable<LayoutNode["lane"]>, number> = {
      configuration: 0,
      main: 180,
      shared: 540,
      test: 760,
      tooling: 980,
    }
    const grouped = new Map<string, LayoutNode[]>()
    for (const node of nodes) {
      const lane = node.lane ?? "main"
      const rank = node.rank ?? 3
      const key = `${laneOrder[lane]}:${rank}`
      grouped.set(key, [...(grouped.get(key) ?? []), node])
    }
    const result: Record<string, GraphPosition> = {}
    for (const [key, group] of [...grouped].sort(([left], [right]) => left.localeCompare(right))) {
      const [laneValue, rankValue] = key.split(":").map(Number)
      const lane = (Object.keys(laneOrder) as Array<keyof typeof laneOrder>)
        .find((candidate) => laneOrder[candidate] === laneValue) ?? "main"
      group.sort((left, right) => left.id.localeCompare(right.id)).forEach((node, index) => {
        result[node.id] = {
          x: rankValue * 286,
          y: laneBase[lane] + index * 132,
        }
      })
    }
    return Promise.resolve(result)
  }

  return elk.layout({
    id: "audit-graph",
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "RIGHT",
      "elk.spacing.nodeNode": "36",
      "elk.layered.spacing.nodeNodeBetweenLayers": "90",
      "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX",
    },
    children: nodes.map((node) => ({ id: node.id, width: node.width ?? 184, height: node.height ?? 68 })),
    edges: edges.map((edge) => ({
      id: edge.id,
      sources: [edge.source],
      targets: [edge.target],
    })),
  }).then((graph) => Object.fromEntries(
    (graph.children ?? []).map((node) => [node.id, { x: node.x ?? 0, y: node.y ?? 0 }]),
  ))
}
