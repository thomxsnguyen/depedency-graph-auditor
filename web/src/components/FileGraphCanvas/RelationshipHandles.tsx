import { Fragment } from "react"
import { Handle, Position } from "@xyflow/react"
import { FILE_RELATIONSHIP_DETAILS } from "../../graph/hierarchicalFileGraph"

export function RelationshipHandles({ className }: { className: string }) {
  return FILE_RELATIONSHIP_DETAILS.map(({ relationship }, index) => (
    <Fragment key={relationship}>
      <Handle
        id={`target-${relationship}`}
        type="target"
        position={Position.Left}
        className={`${className} relationship-handle`}
        style={{ top: 20 + index * 6 }}
        isConnectable={false}
      />
      <Handle
        id={`source-${relationship}`}
        type="source"
        position={Position.Right}
        className={`${className} relationship-handle`}
        style={{ top: 50 + index * 6 }}
        isConnectable={false}
      />
    </Fragment>
  ))
}
