import { memo, useEffect, useRef } from "react"
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react"

export interface CanvasBoxNodeData extends Record<string, unknown> {
  label: string
  editing: boolean
  onBeginEdit?: (boxId: string) => void
  onCommitEdit?: (boxId: string, label: string) => void
  onCancelEdit?: (boxId: string) => void
}

export type CanvasBoxGraphNode = Node<CanvasBoxNodeData, "canvasBox">

export const CanvasBoxNode = memo(function CanvasBoxNode({
  id,
  data,
  selected,
  isConnectable,
}: NodeProps<CanvasBoxGraphNode>) {
  const inputRef = useRef<HTMLInputElement>(null)
  const finishedRef = useRef(false)

  useEffect(() => {
    if (!data.editing) return
    finishedRef.current = false
    const frame = window.requestAnimationFrame(() => {
      inputRef.current?.focus()
      inputRef.current?.select()
    })
    return () => window.cancelAnimationFrame(frame)
  }, [data.editing])

  const commit = () => {
    if (finishedRef.current) return
    finishedRef.current = true
    data.onCommitEdit?.(id, inputRef.current?.value ?? data.label)
  }

  const cancel = () => {
    if (finishedRef.current) return
    finishedRef.current = true
    data.onCancelEdit?.(id)
  }

  return (
    <div className={`canvas-box ${selected ? "canvas-box--selected" : ""}`}>
      <Handle
        type="target"
        position={Position.Left}
        className="canvas-box__handle canvas-box__handle--target"
        isConnectable={isConnectable}
      />
      {data.editing ? (
        <input
          ref={inputRef}
          className="canvas-box__input nodrag nopan"
          type="text"
          defaultValue={data.label}
          aria-label="Presentation box label"
          onBlur={commit}
          onKeyDown={(event) => {
            event.stopPropagation()
            if (event.key === "Enter") {
              event.preventDefault()
              commit()
            } else if (event.key === "Escape") {
              event.preventDefault()
              cancel()
            }
          }}
        />
      ) : (
        <div
          className="canvas-box__label"
          role="button"
          tabIndex={0}
          aria-label={`${data.label}, presentation box`}
          onDoubleClick={() => data.onBeginEdit?.(id)}
          onKeyDown={(event) => {
            if (event.key === "Enter") data.onBeginEdit?.(id)
          }}
        >
          {data.label}
        </div>
      )}
      <Handle
        type="source"
        position={Position.Right}
        className="canvas-box__handle canvas-box__handle--source"
        isConnectable={isConnectable}
      />
    </div>
  )
})
