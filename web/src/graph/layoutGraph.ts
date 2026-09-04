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
    const grouped = new Map<string, LayoutNode[]>()
    for (const node of nodes) {
      const lane = node.lane ?? "main"
      const rank = node.rank ?? 3
      const key = `${laneOrder[lane]}:${rank}`
      grouped.set(key, [...(grouped.get(key) ?? []), node])
    }
    const rankWidth = new Map<number, number>()
    for (const node of nodes) {
      const rank = node.rank ?? 3
      rankWidth.set(rank, Math.max(rankWidth.get(rank) ?? 0, node.width ?? 300))
    }
    const rankX = new Map<number, number>()
    let nextX = 0
    for (const rank of [...rankWidth.keys()].sort((left, right) => left - right)) {
      rankX.set(rank, nextX)
      nextX += (rankWidth.get(rank) ?? 300) + 72
    }
    const laneHeight: Record<NonNullable<LayoutNode["lane"]>, number> = {
      configuration: 0,
      main: 0,
      shared: 0,
      test: 0,
      tooling: 0,
    }
    for (const [key, group] of grouped) {
      const laneValue = Number(key.split(":")[0])
      const lane = (Object.keys(laneOrder) as Array<keyof typeof laneOrder>)
        .find((candidate) => laneOrder[candidate] === laneValue) ?? "main"
      const height = group.reduce((total, node) => total + Math.max(132, (node.height ?? 96) + 36), 0)
      laneHeight[lane] = Math.max(laneHeight[lane], height)
    }
    const laneBase: Record<NonNullable<LayoutNode["lane"]>, number> = {
      configuration: 0,
      main: Math.max(180, laneHeight.configuration + 84),
      shared: 0,
      test: 0,
      tooling: 0,
    }
    laneBase.shared = laneBase.main + Math.max(276, laneHeight.main) + 84
    laneBase.test = laneBase.shared + Math.max(136, laneHeight.shared) + 84
    laneBase.tooling = laneBase.test + Math.max(136, laneHeight.test) + 84
    const result: Record<string, GraphPosition> = {}
    for (const [key, group] of [...grouped].sort(([left], [right]) => left.localeCompare(right))) {
      const [laneValue, rankValue] = key.split(":").map(Number)
      const lane = (Object.keys(laneOrder) as Array<keyof typeof laneOrder>)
        .find((candidate) => laneOrder[candidate] === laneValue) ?? "main"
      let laneOffset = 0
      group.sort((left, right) => left.id.localeCompare(right.id)).forEach((node) => {
        result[node.id] = {
          x: rankX.get(rankValue) ?? rankValue * 372,
          y: laneBase[lane] + laneOffset,
        }
        laneOffset += Math.max(132, (node.height ?? 96) + 36)
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
