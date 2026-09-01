import { Search, X } from "lucide-react"
import type { AuditCounts } from "../../types/audit"
import type { GraphFilters } from "../../types/graphView"

interface AuditSidebarProps {
  packageCount: number
  violationCount: number
  counts: AuditCounts
  filters: GraphFilters
  open: boolean
  onClose: () => void
  onSearch: (value: string) => void
  onFilter: (name: "violationsOnly" | "completeGraph", value: boolean) => void
}

export function AuditSidebar({
  packageCount,
  violationCount,
  counts,
  filters,
  open,
  onClose,
  onSearch,
  onFilter,
}: AuditSidebarProps) {
  return (
    <aside className={`sidebar sidebar--left ${open ? "sidebar--open" : ""}`} aria-label="Audit summary and filters">
      <div className="sidebar__mobile-header">
        <strong>Audit summary</strong>
        <button className="icon-button" type="button" onClick={onClose} aria-label="Close audit summary">
          <X size={18} aria-hidden="true" />
        </button>
      </div>

      <div>
        <p className="eyebrow">Audit summary</p>
        <div className="summary-line">
          <strong>{packageCount}</strong>
          <span>packages discovered</span>
        </div>
      </div>

      <div className="summary-list" aria-label="Audit results">
        <span><i className="status-dot status-dot--pass" />{packageCount - violationCount} passed policy</span>
        <span><i className="status-dot status-dot--violation" />{violationCount} policy violation</span>
        <span><i className="status-dot status-dot--retrying" />{counts.retrying} retrying</span>
        <span><i className="status-dot status-dot--dead" />{counts.dead_lettered} dead-lettered</span>
      </div>

      <label className="search-field">
        <span>Search packages</span>
        <span className="search-field__control">
          <Search size={16} aria-hidden="true" />
          <input
            type="search"
            placeholder="Name or version"
            value={filters.search}
            onChange={(event) => onSearch(event.target.value)}
          />
        </span>
      </label>

      <fieldset className="filter-group">
        <legend>Show</legend>
        <label>
          <input
            type="checkbox"
            checked={filters.violationsOnly}
            onChange={(event) => onFilter("violationsOnly", event.target.checked)}
          />
          Violations only
        </label>
        <label>
          <input
            type="checkbox"
            checked={filters.completeGraph}
            onChange={(event) => onFilter("completeGraph", event.target.checked)}
          />
          Entire dependency graph
        </label>
      </fieldset>

      <div className="sidebar__hint">
        <span>Tip</span>
        <p>Select a package to trace its shortest path from the project root.</p>
      </div>
    </aside>
  )
}
