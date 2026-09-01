import type { GraphViewStore } from "./GraphViewStore"
import { initialGraphView, type CanvasArrow, type CanvasBox, type GraphPosition, type GraphView } from "../types/graphView"

// Version the demo presentation state so incompatible viewport/layout changes
// cannot strand the graph outside the visible canvas after an upgrade.
const keyForAudit = (auditId: string) => `dependency-audit-view:v4:${auditId}`

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function readPosition(value: unknown): GraphPosition | null {
  if (!isRecord(value)) return null
  return typeof value.x === "number" && Number.isFinite(value.x)
    && typeof value.y === "number" && Number.isFinite(value.y)
    ? { x: value.x, y: value.y }
    : null
}

function readStringRecord(value: unknown): Record<string, string> {
  if (!isRecord(value)) return {}
  return Object.fromEntries(
    Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === "string"),
  )
}

function readPositionRecord(value: unknown): Record<string, GraphPosition> {
  if (!isRecord(value)) return {}
  return Object.fromEntries(
    Object.entries(value).flatMap(([id, position]) => {
      const parsed = readPosition(position)
      return parsed ? [[id, parsed] as const] : []
    }),
  )
}

function readCanvasBoxes(value: unknown): CanvasBox[] {
  if (!Array.isArray(value)) return []
  const ids = new Set<string>()
  const boxes: CanvasBox[] = []
  for (const candidate of value) {
    if (!isRecord(candidate) || typeof candidate.id !== "string" || ids.has(candidate.id)) continue
    if (typeof candidate.label !== "string") continue
    const position = readPosition(candidate.position)
    if (!position) continue
    ids.add(candidate.id)
    boxes.push({ id: candidate.id, label: candidate.label.trim() || "Untitled", position })
  }
  return boxes
}

function readCanvasArrows(value: unknown, boxIds: Set<string>): CanvasArrow[] {
  if (!Array.isArray(value)) return []
  const ids = new Set<string>()
  const connections = new Set<string>()
  const arrows: CanvasArrow[] = []
  for (const candidate of value) {
    if (
      !isRecord(candidate)
      || typeof candidate.id !== "string"
      || typeof candidate.sourceBoxId !== "string"
      || typeof candidate.targetBoxId !== "string"
      || ids.has(candidate.id)
      || candidate.sourceBoxId === candidate.targetBoxId
      || !boxIds.has(candidate.sourceBoxId)
      || !boxIds.has(candidate.targetBoxId)
    ) continue
    const connection = `${candidate.sourceBoxId}\u0000${candidate.targetBoxId}`
    if (connections.has(connection)) continue
    ids.add(candidate.id)
    connections.add(connection)
    arrows.push({
      id: candidate.id,
      sourceBoxId: candidate.sourceBoxId,
      targetBoxId: candidate.targetBoxId,
    })
  }
  return arrows
}

function readViewport(value: unknown): GraphView["viewport"] {
  if (!isRecord(value)) return null
  return typeof value.x === "number" && Number.isFinite(value.x)
    && typeof value.y === "number" && Number.isFinite(value.y)
    && typeof value.zoom === "number" && Number.isFinite(value.zoom)
    ? { x: value.x, y: value.y, zoom: value.zoom }
    : null
}

function readGraphView(value: unknown): GraphView | null {
  if (!isRecord(value)) return null
  const canvasBoxes = readCanvasBoxes(value.canvasBoxes)
  const boxIds = new Set(canvasBoxes.map((box) => box.id))
  const filters = isRecord(value.filters) ? value.filters : {}
  return {
    ...initialGraphView,
    selectedNodeId: typeof value.selectedNodeId === "string" ? value.selectedNodeId : null,
    filters: {
      search: typeof filters.search === "string" ? filters.search : "",
      violationsOnly: typeof filters.violationsOnly === "boolean" ? filters.violationsOnly : false,
      completeGraph: typeof filters.completeGraph === "boolean" ? filters.completeGraph : false,
    },
    pinnedPositions: readPositionRecord(value.pinnedPositions),
    collapsedNodeIds: Array.isArray(value.collapsedNodeIds)
      ? value.collapsedNodeIds.filter((id): id is string => typeof id === "string")
      : [],
    annotations: readStringRecord(value.annotations),
    canvasBoxes,
    canvasArrows: readCanvasArrows(value.canvasArrows, boxIds),
    viewport: readViewport(value.viewport),
    summaryOpen: typeof value.summaryOpen === "boolean" ? value.summaryOpen : false,
    inspectorOpen: typeof value.inspectorOpen === "boolean" ? value.inspectorOpen : false,
    activityOpen: typeof value.activityOpen === "boolean" ? value.activityOpen : false,
  }
}

export class LocalGraphViewStore implements GraphViewStore {
  async load(auditId: string): Promise<GraphView | null> {
    const raw = window.localStorage.getItem(keyForAudit(auditId))
    if (!raw) return null

    try {
      return readGraphView(JSON.parse(raw))
    } catch {
      return null
    }
  }

  async save(auditId: string, view: GraphView): Promise<void> {
    window.localStorage.setItem(keyForAudit(auditId), JSON.stringify(view))
  }
}
