import ELK from "elkjs/lib/elk-api.js"
import ElkWorker from "elkjs/lib/elk-worker.min.js?worker"
import type { GraphPosition } from "../types/graphView"

const elk = new ELK({
  workerFactory: () => new ElkWorker(),
})

export function layoutGraph(
  nodes: readonly { id: string }[],
  edges: readonly { id: string; source: string; target: string }[],
): Promise<Record<string, GraphPosition>> {
  if (nodes.length === 0) return Promise.resolve({})

  return elk.layout({
    id: "audit-graph",
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "RIGHT",
      "elk.spacing.nodeNode": "36",
      "elk.layered.spacing.nodeNodeBetweenLayers": "90",
      "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX",
    },
    children: nodes.map((node) => ({ id: node.id, width: 184, height: 68 })),
    edges: edges.map((edge) => ({
      id: edge.id,
      sources: [edge.source],
      targets: [edge.target],
    })),
  }).then((graph) => Object.fromEntries(
    (graph.children ?? []).map((node) => [node.id, { x: node.x ?? 0, y: node.y ?? 0 }]),
  ))
}
