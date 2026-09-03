import { Search, X } from "lucide-react"

interface FileSidebarProps {
  fileCount: number
  importCount: number
  diagnosticCount: number
  search: string
  open: boolean
  onClose: () => void
  onSearch: (value: string) => void
}

export function FileSidebar({
  fileCount,
  importCount,
  diagnosticCount,
  search,
  open,
  onClose,
  onSearch,
}: FileSidebarProps) {
  return (
    <aside className={`sidebar sidebar--left ${open ? "sidebar--open" : ""}`} aria-label="File graph summary and search">
      <div className="sidebar__mobile-header">
        <strong>File graph</strong>
        <button className="icon-button" type="button" onClick={onClose} aria-label="Close file graph summary">
          <X size={18} aria-hidden="true" />
        </button>
      </div>

      <div>
        <p className="eyebrow">File graph</p>
        <div className="summary-line">
          <strong>{fileCount}</strong>
          <span>files discovered</span>
        </div>
      </div>

      <div className="summary-list" aria-label="File graph totals">
        <span><i className="summary-mark" />{importCount} resolved imports</span>
        <span><i className="summary-mark summary-mark--warning" />{diagnosticCount} diagnostics</span>
      </div>

      <label className="search-field">
        <span>Search files</span>
        <span className="search-field__control">
          <Search size={16} aria-hidden="true" />
          <input
            type="search"
            placeholder="Repository-relative path"
            value={search}
            onChange={(event) => onSearch(event.target.value)}
          />
          {search && (
            <button className="search-field__clear" type="button" onClick={() => onSearch("")} aria-label="Clear file search">
              <X size={14} aria-hidden="true" />
            </button>
          )}
        </span>
      </label>

      <div className="sidebar__hint">
        <span>Tip</span>
        <p>Select a file to inspect what it imports and which files import it.</p>
      </div>
    </aside>
  )
}
