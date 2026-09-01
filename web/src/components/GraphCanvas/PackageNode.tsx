import { memo } from "react"
import { ChevronRight, MessageSquareText } from "lucide-react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import type { AuditGraphNode } from "../../graph/mapAuditGraph"
import { StatusIndicator } from "../primitives/StatusIndicator"

export const PackageNode = memo(function PackageNode({ id, data, selected }: NodeProps<AuditGraphNode>) {
  const packageRow = data.packageRow

  return (
    <div
      className={[
        "package-node",
        data.root ? "package-node--root" : "",
        data.selectedPath ? "package-node--path" : "",
        selected ? "package-node--selected" : "",
      ].filter(Boolean).join(" ")}
    >
      <Handle type="target" position={Position.Left} className="package-node__handle" />
      <div
        className="package-node__content"
        role="button"
        tabIndex={0}
        aria-label={data.root ? `${data.label}, audit root` : `${data.label}, ${packageRow?.status.replaceAll("_", " ")}`}
        onClick={() => data.onSelect?.(id)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault()
            data.onSelect?.(id)
          }
        }}
      >
        <div className="package-node__heading">
          <div>
            <strong>{packageRow?.name ?? data.label}</strong>
            {packageRow && <small>{packageRow.version} · {packageRow.license || "Unknown license"}</small>}
            {data.root && <small>Audit root</small>}
          </div>
          {packageRow && <StatusIndicator status={packageRow.status} verdict={packageRow.verdict} compact />}
        </div>
        <div className="package-node__footer">
          <span>{data.collapsed ? "Branch collapsed" : packageRow?.verdict === "policy_violation" ? "Review required" : "Dependency"}</span>
          {data.annotated && <MessageSquareText size={13} aria-label="Has annotation" />}
        </div>
      </div>
      <button
        className="node-action nodrag nopan"
        type="button"
        onClick={(event) => {
          event.stopPropagation()
          data.onCollapse?.(id)
        }}
        aria-label={data.collapsed ? `Expand ${data.label} branch` : `Collapse ${data.label} branch`}
      >
        <ChevronRight size={14} aria-hidden="true" className={data.collapsed ? "" : "node-action__expanded"} />
      </button>
      <Handle type="source" position={Position.Right} className="package-node__handle" />
    </div>
  )
})
