import { memo } from "react"
import { AlertTriangle, ChevronRight, FolderTree } from "lucide-react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import type { ModuleFlowNode } from "../../graph/mapFileGraph"

export const ModuleNode = memo(function ModuleNode({ data, selected }: NodeProps<ModuleFlowNode>) {
  const hasDiagnostics = data.diagnosticCount > 0
  return (
    <div className={`module-node ${hasDiagnostics ? "module-node--diagnostic" : ""} ${selected || data.selected ? "module-node--selected" : ""}`}>
      <Handle type="target" position={Position.Left} className="module-node__handle" isConnectable={false} />
      <div
        className="module-node__content"
        role="button"
        tabIndex={0}
        aria-label={`Select module ${data.path}`}
        onClick={() => data.onSelect?.(data.path)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault()
            data.onSelect?.(data.path)
          }
        }}
      >
        <FolderTree className="module-node__icon" size={17} aria-hidden="true" />
        <span className="module-node__labels">
          <strong>{data.path === "." ? "Repository root" : data.path}</strong>
          <small>{data.fileCount} {data.fileCount === 1 ? "file" : "files"} · {data.internalDependencyCount} internal</small>
        </span>
        {hasDiagnostics && (
          <AlertTriangle className="module-node__warning" size={14} aria-label={`${data.diagnosticCount} diagnostics`} />
        )}
      </div>
      <button
        className="module-node__toggle nodrag nopan"
        type="button"
        aria-label={`Expand module ${data.path}`}
        onClick={(event) => {
          event.stopPropagation()
          data.onToggle?.(data.path)
        }}
      >
        <ChevronRight size={14} aria-hidden="true" />
        <span>Expand</span>
      </button>
      <Handle type="source" position={Position.Right} className="module-node__handle" isConnectable={false} />
    </div>
  )
})
