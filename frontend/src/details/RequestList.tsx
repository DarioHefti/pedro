import { useEffect, useMemo, useRef } from 'react'
import type { KeyboardEvent } from 'react'
import { formatFull, formatTime, groupByDay } from './payload'
import type { RequestRow } from './types'

/* -------------------------------------------------------------------------- */
/*  Single row                                                                 */
/* -------------------------------------------------------------------------- */

interface RequestListItemProps {
  row: RequestRow
  active: boolean
  onSelect: () => void
}

function RequestListItem({ row, active, onSelect }: RequestListItemProps) {
  const { entry, payload } = row
  const title = entry.ConversationTitle || 'Untitled conversation'
  const hasReply = payload.response.length > 0

  return (
    <button
      type="button"
      role="option"
      aria-selected={active}
      data-row-id={entry.ID}
      className={`rd-row${active ? ' rd-row--active' : ''}`}
      onClick={onSelect}
      title={title}
    >
      <span className="rd-row-head">
        <span className="rd-row-title">{title}</span>
        <time className="rd-row-time" title={formatFull(row.createdAt)}>
          {formatTime(row.createdAt)}
        </time>
      </span>
      <span className="rd-row-preview">{row.preview}</span>
      <span className="rd-row-meta">
        {entry.Model && <span className="rd-row-model">{entry.Model}</span>}
        <span className="rd-row-stat">{payload.messages.length} msgs</span>
        {payload.tools.length > 0 && <span className="rd-row-stat">{payload.tools.length} tools</span>}
        <span
          className={`rd-status-dot${hasReply ? ' rd-status-dot--ok' : ''}`}
          title={hasReply ? 'Reply captured' : 'No reply captured'}
        />
      </span>
    </button>
  )
}

/* -------------------------------------------------------------------------- */
/*  Search field                                                               */
/* -------------------------------------------------------------------------- */

interface SearchFieldProps {
  value: string
  onChange: (v: string) => void
  shown: number
  total: number
  onKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void
}

function SearchField({ value, onChange, shown, total, onKeyDown }: SearchFieldProps) {
  return (
    <div className="rd-search">
      <svg className="rd-search-icon" viewBox="0 0 20 20" aria-hidden="true">
        <circle cx="9" cy="9" r="5.5" />
        <path d="M13.2 13.2 17 17" />
      </svg>
      <input
        type="text"
        className="rd-search-input"
        placeholder="Search requests…"
        value={value}
        onChange={e => onChange(e.target.value)}
        onKeyDown={onKeyDown}
        spellCheck={false}
        aria-label="Search requests by model, role or content"
      />
      {value ? (
        <button type="button" className="rd-search-clear" onClick={() => onChange('')} aria-label="Clear search">
          ✕
        </button>
      ) : null}
      <span className="rd-search-count" aria-live="polite">
        {value ? `${shown}/${total}` : total}
      </span>
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/*  List pane                                                                  */
/* -------------------------------------------------------------------------- */

interface RequestListProps {
  rows: RequestRow[]
  totalCount: number
  loading: boolean
  filter: string
  onFilterChange: (v: string) => void
  selectedId: number | null
  onSelect: (id: number) => void
}

export function RequestList({
  rows,
  totalCount,
  loading,
  filter,
  onFilterChange,
  selectedId,
  onSelect,
}: RequestListProps) {
  const listRef = useRef<HTMLDivElement>(null)
  const groups = useMemo(() => groupByDay(rows), [rows])

  // Keep the active row visible (e.g. when auto-selected or navigated by keyboard).
  useEffect(() => {
    if (selectedId == null) return
    const el = listRef.current?.querySelector<HTMLElement>(`[data-row-id="${selectedId}"]`)
    el?.scrollIntoView({ block: 'nearest' })
  }, [selectedId])

  function handleKeyDown(e: KeyboardEvent<HTMLElement>) {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    if (rows.length === 0) return
    e.preventDefault()
    const idx = rows.findIndex(r => r.entry.ID === selectedId)
    const next = e.key === 'ArrowDown'
      ? Math.min(rows.length - 1, idx + 1)
      : Math.max(0, idx - 1)
    onSelect(rows[next].entry.ID)
  }

  return (
    <aside className="rd-list-pane">
      <SearchField
        value={filter}
        onChange={onFilterChange}
        shown={rows.length}
        total={totalCount}
        onKeyDown={handleKeyDown}
      />

      <div
        ref={listRef}
        className="rd-list"
        role="listbox"
        aria-label="Captured requests"
        tabIndex={-1}
        onKeyDown={handleKeyDown}
      >
        {loading ? (
          Array.from({ length: 6 }).map((_, i) => <div key={i} className="rd-skeleton" />)
        ) : rows.length === 0 ? (
          <div className="rd-list-empty">
            {totalCount === 0 ? 'No requests recorded yet' : `No matches for “${filter.trim()}”`}
          </div>
        ) : (
          groups.map(group => (
            <section key={group.key} className="rd-group">
              <h3 className="rd-group-label">
                <span>{group.label}</span>
                <span className="rd-group-count">{group.rows.length}</span>
              </h3>
              {group.rows.map(r => (
                <RequestListItem
                  key={r.entry.ID}
                  row={r}
                  active={r.entry.ID === selectedId}
                  onSelect={() => onSelect(r.entry.ID)}
                />
              ))}
            </section>
          ))
        )}
      </div>
    </aside>
  )
}
