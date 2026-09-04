import { memo } from "react"
import { AlertTriangle, ChevronRight, Layers3 } from "lucide-react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import type { ArchitectureFlowNode } from "../../graph/mapFileGraph"

export const ArchitectureNode = memo(function ArchitectureNode({ data, selected }: NodeProps<ArchitectureFlowNode>) {
  const hasDiagnostics = data.diagnosticCount > 0
  return (
    <div className={`architecture-node architecture-node--${data.lane} ${data.expanded ? "architecture-node--expanded" : ""} ${hasDiagnostics ? "architecture-node--diagnostic" : ""} ${selected || data.selected ? "architecture-node--selected" : ""}`}>
      <Handle type="target" position={Position.Left} className="architecture-node__handle" isConnectable={false} />
      <div
        className="architecture-node__content"
        role="button"
        tabIndex={0}
        aria-label={`Select ${data.label} in ${data.project}`}
        onClick={() => data.onSelect?.(data.id)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault()
            data.onSelect?.(data.id)
          }
        }}
      >
        <Layers3 className="architecture-node__icon" size={17} aria-hidden="true" />
        <span className="architecture-node__labels">
          <strong>{data.label}</strong>
          <small>{data.project}</small>
        </span>
        {hasDiagnostics && (
          <AlertTriangle className="architecture-node__warning" size={14} aria-label={`${data.diagnosticCount} diagnostics`} />
        )}
      </div>
      <div className="architecture-node__footer">
        <span>{data.domainCount} {data.domainCount === 1 ? "folder" : "folders"} · {data.fileCount} {data.fileCount === 1 ? "file" : "files"}</span>
        <button
          className="architecture-node__toggle nodrag nopan"
          type="button"
          aria-label={`${data.expanded ? "Collapse" : "Expand"} ${data.label} in ${data.project}`}
          aria-expanded={data.expanded}
          onClick={(event) => {
            event.stopPropagation()
            data.onToggle?.(data.id, data.project)
          }}
        >
          <ChevronRight className={data.expanded ? "architecture-node__chevron--expanded" : ""} size={14} aria-hidden="true" />
          <span>{data.expanded ? "Hide" : "Folders"}</span>
        </button>
      </div>
      <Handle type="source" position={Position.Right} className="architecture-node__handle" isConnectable={false} />
    </div>
  )
})
