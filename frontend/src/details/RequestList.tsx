import { useMemo } from 'react'
import type { KeyboardEvent } from 'react'
import { Inbox, Search, X } from 'lucide-react'
import { Input, InputClear, InputGroup } from '@ui-sdk/react/input'
import { Listbox, ListboxGroup, ListboxGroupLabel, ListboxItem } from '@ui-sdk/react/listbox'
import { formatFull, formatTime, groupByDay } from './payload'
import type { RequestRow } from './types'

function RequestListItem({ row }: { row: RequestRow }) {
  const { entry, payload } = row
  const title = entry.ConversationTitle || 'Untitled conversation'
  const hasReply = payload.response.length > 0

  return (
    <ListboxItem value={String(entry.ID)} label={title} className="rd-row">
      <span className="rd-row-head">
        <span className="rd-row-title">{title}</span>
        <time className="rd-row-time" title={formatFull(row.createdAt)}>
          {formatTime(row.createdAt)}
        </time>
      </span>
      <span className="rd-row-preview">{row.preview}</span>
      <span className="rd-row-meta">
        {entry.Model ? <span className="rd-row-model">{entry.Model}</span> : null}
        <span className="rd-row-stat">{payload.messages.length} msgs</span>
        {payload.tools.length > 0 ? <span className="rd-row-stat">{payload.tools.length} tools</span> : null}
        <span
          className="status-dot rd-row-dot"
          data-tone={hasReply ? 'success' : undefined}
          title={hasReply ? 'Reply captured' : 'No reply captured'}
        />
      </span>
    </ListboxItem>
  )
}

interface RequestListProps {
  rows: RequestRow[]
  totalCount: number
  loading: boolean
  filter: string
  onFilterChange: (v: string) => void
  selectedId: number | null
  onSelect: (id: number) => void
}

export function RequestList({ rows, totalCount, loading, filter, onFilterChange, selectedId, onSelect }: RequestListProps) {
  const groups = useMemo(() => groupByDay(rows), [rows])

  function handleSearchKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if ((e.key !== 'ArrowDown' && e.key !== 'ArrowUp') || rows.length === 0) return
    e.preventDefault()
    const idx = rows.findIndex(r => r.entry.ID === selectedId)
    const next = e.key === 'ArrowDown' ? Math.min(rows.length - 1, idx + 1) : Math.max(0, idx - 1)
    onSelect(rows[next].entry.ID)
  }

  return (
    <aside className="rd-list-pane">
      <div className="rd-search">
        <InputGroup>
          <Search className="input-leading-icon" aria-hidden="true" />
          <Input
            value={filter}
            onValueChange={v => onFilterChange(v)}
            onKeyDown={handleSearchKeyDown}
            placeholder="Search requests"
            aria-label="Search requests by model, role or content"
            spellCheck={false}
          />
          <InputClear aria-label="Clear search">
            <X aria-hidden="true" />
          </InputClear>
        </InputGroup>
        <span className="rd-search-count" aria-live="polite">
          {filter ? `${rows.length}/${totalCount}` : totalCount}
        </span>
      </div>

      {loading ? (
        <div className="rd-list">
          {Array.from({ length: 6 }, (_, i) => (
            <div key={`skeleton-${i}`} className="skeleton rd-skeleton" />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <div className="empty-hint rd-list-empty">
          {totalCount === 0 ? <Inbox aria-hidden="true" /> : <Search aria-hidden="true" />}
          <span>{totalCount === 0 ? 'No requests recorded yet' : `No matches for “${filter.trim()}”`}</span>
        </div>
      ) : (
        <Listbox<string>
          className="rd-list"
          aria-label="Captured requests"
          value={selectedId != null ? String(selectedId) : null}
          onValueChange={v => {
            if (v != null) onSelect(Number(v))
          }}
        >
          {groups.map(group => (
            <ListboxGroup key={group.key} className="rd-group">
              <ListboxGroupLabel className="rd-group-label">
                <span>{group.label}</span>
                <span className="rd-group-count">{group.rows.length}</span>
              </ListboxGroupLabel>
              {group.rows.map(r => (
                <RequestListItem key={r.entry.ID} row={r} />
              ))}
            </ListboxGroup>
          ))}
        </Listbox>
      )}
    </aside>
  )
}
