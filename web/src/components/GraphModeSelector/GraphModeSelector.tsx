import { useRef, type KeyboardEvent } from "react"
import type { GraphMode } from "../../types/fileGraph"

interface GraphModeSelectorProps {
  mode: GraphMode
  onChange: (mode: GraphMode) => void
}

export function GraphModeSelector({ mode, onChange }: GraphModeSelectorProps) {
  const dependencyRef = useRef<HTMLButtonElement>(null)
  const filesRef = useRef<HTMLButtonElement>(null)

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return
    event.preventDefault()
    const nextMode: GraphMode = mode === "dependencies" ? "files" : "dependencies"
    onChange(nextMode)
    window.requestAnimationFrame(() => {
      if (nextMode === "dependencies") dependencyRef.current?.focus()
      else filesRef.current?.focus()
    })
  }

  return (
    <div className="graph-mode-selector" role="tablist" aria-label="Graph view">
      <button
        ref={dependencyRef}
        role="tab"
        type="button"
        aria-selected={mode === "dependencies"}
        tabIndex={mode === "dependencies" ? 0 : -1}
        onClick={() => onChange("dependencies")}
        onKeyDown={onKeyDown}
      >
        Dependencies
      </button>
      <button
        ref={filesRef}
        role="tab"
        type="button"
        aria-selected={mode === "files"}
        tabIndex={mode === "files" ? 0 : -1}
        onClick={() => onChange("files")}
        onKeyDown={onKeyDown}
      >
        Files
      </button>
    </div>
  )
}
