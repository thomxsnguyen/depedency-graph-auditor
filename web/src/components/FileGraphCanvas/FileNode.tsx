import { memo } from "react"
import { AlertTriangle, FileCode2 } from "lucide-react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import type { FileFlowNode } from "../../graph/mapFileGraph"
import { categoryDetails } from "../../graph/fileCategory"

export const FileNode = memo(function FileNode({ data, selected }: NodeProps<FileFlowNode>) {
  const category = categoryDetails(data.category)
  const hasDiagnostics = data.diagnosticCount > 0
  return (
    <div className={`file-node file-node--${data.category} ${hasDiagnostics ? "file-node--diagnostic" : ""} ${selected || data.selected ? "file-node--selected" : ""}`}>
      <Handle type="target" position={Position.Left} className="file-node__handle" isConnectable={false} />
      <div
        className="file-node__content"
        role="button"
        tabIndex={0}
        aria-label={`${data.path}, ${category.label}${hasDiagnostics ? `, ${data.diagnosticCount} diagnostics` : ""}`}
        onClick={() => data.onSelect?.(data.path)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault()
            data.onSelect?.(data.path)
          }
        }}
      >
        <FileCode2 className="file-node__icon" size={16} aria-hidden="true" />
        <span className="file-node__labels">
          <strong>{data.fileName}</strong>
          <small>
            <span className="file-node__path">{data.parentPath}</span>
            <span className="file-node__category">{category.label}</span>
          </small>
        </span>
        {hasDiagnostics && (
          <AlertTriangle className="file-node__warning" size={14} aria-label={`${data.diagnosticCount} diagnostics`} />
        )}
      </div>
      <Handle type="source" position={Position.Right} className="file-node__handle" isConnectable={false} />
    </div>
  )
})
