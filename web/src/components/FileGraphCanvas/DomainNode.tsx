import { memo } from "react"
import { AlertTriangle, ChevronRight, FolderTree } from "lucide-react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import type { DomainFlowNode } from "../../graph/mapFileGraph"

export const DomainNode = memo(function DomainNode({ data, selected }: NodeProps<DomainFlowNode>) {
  const hasDiagnostics = data.diagnosticCount > 0
  return (
    <div className={`domain-node domain-node--${data.lane} ${hasDiagnostics ? "domain-node--diagnostic" : ""} ${selected || data.selected ? "domain-node--selected" : ""}`}>
      <Handle type="target" position={Position.Left} className="domain-node__handle" isConnectable={false} />
      <div
        className="domain-node__content"
        role="button"
        tabIndex={0}
        aria-label={`Select domain ${data.domain} in ${data.layerLabel}`}
        onClick={() => data.onSelect?.(data.id)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault()
            data.onSelect?.(data.id)
          }
        }}
      >
        <FolderTree className="domain-node__icon" size={17} aria-hidden="true" />
        <span className="domain-node__labels">
          <strong>{data.domain}</strong>
          <small>{data.layerLabel} · {data.project}</small>
        </span>
        {hasDiagnostics && (
          <AlertTriangle className="domain-node__warning" size={14} aria-label={`${data.diagnosticCount} diagnostics`} />
        )}
      </div>
      <div className="domain-node__footer">
        <span>{data.fileCount} {data.fileCount === 1 ? "file" : "files"} · {data.internalDependencyCount} internal</span>
        <button
          className="domain-node__toggle nodrag nopan"
          type="button"
          aria-label={`Expand domain ${data.domain} in ${data.layerLabel}`}
          onClick={(event) => {
            event.stopPropagation()
            data.onToggle?.(data.id, data.architectureId)
          }}
        >
          <ChevronRight size={14} aria-hidden="true" />
          <span>Files</span>
        </button>
      </div>
      <Handle type="source" position={Position.Right} className="domain-node__handle" isConnectable={false} />
    </div>
  )
})
