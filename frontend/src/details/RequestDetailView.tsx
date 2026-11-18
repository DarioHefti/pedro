import { useState } from 'react'
import type { ReactNode } from 'react'
import { CopyButton } from './DetailsButton'
import { MessageTimeline } from './MessageTimeline'
import { ParamsTable } from './ParamsTable'
import { RawJsonView } from './RawJsonView'
import { ToolDefinitionList } from './ToolDefinitionList'
import { ROLE_ORDER, compactNumber, formatFull, humanSize, roleLabel } from './payload'
import type { ParsedPayload, RequestRow } from './types'

type DetailTab = 'conversation' | 'tools' | 'params' | 'raw'

/* -------------------------------------------------------------------------- */
/*  Stats strip                                                                */
/* -------------------------------------------------------------------------- */

function RoleBreakdownBar({ payload }: { payload: ParsedPayload }) {
  const total = payload.messages.length
  if (total === 0) return null
  return (
    <span className="rd-rolebar" aria-hidden="true">
      {ROLE_ORDER.filter(r => payload.roleCounts[r] > 0).map(r => (
        <span
          key={r}
          className={`rd-rolebar-seg rd-rolebar-seg--${r}`}
          style={{ flexGrow: payload.roleCounts[r] }}
          title={`${roleLabel(r)}: ${payload.roleCounts[r]}`}
        />
      ))}
    </span>
  )
}

function StatCard({ label, value, hint, children }: {
  label: string
  value: string
  hint?: string
  children?: ReactNode
}) {
  return (
    <div className="rd-stat">
      <span className="rd-stat-label">{label}</span>
      <span className="rd-stat-value">{value}</span>
      {hint && <span className="rd-stat-hint">{hint}</span>}
      {children}
    </div>
  )
}

function StatsStrip({ payload }: { payload: ParsedPayload }) {
  const breakdown = ROLE_ORDER
    .filter(r => payload.roleCounts[r] > 0)
    .map(r => `${payload.roleCounts[r]} ${roleLabel(r).toLowerCase()}`)
    .join(' · ')

  return (
    <div className="rd-stats">
      <StatCard label="Messages" value={String(payload.messages.length)} hint={breakdown || undefined}>
        <RoleBreakdownBar payload={payload} />
      </StatCard>
      <StatCard label="Prompt size" value={compactNumber(payload.totalChars)} hint="characters" />
      <StatCard label="Tools" value={String(payload.tools.length)} hint={payload.tools.length ? 'function definitions' : 'none'} />
      <StatCard
        label="Response"
        value={payload.response ? compactNumber(payload.response.length) : '—'}
        hint={payload.response ? 'characters' : 'not captured'}
      />
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/*  Detail view                                                                */
/* -------------------------------------------------------------------------- */

export function RequestDetailView({ row }: { row: RequestRow }) {
  const { entry, payload } = row
  const [tab, setTab] = useState<DetailTab>('conversation')
  const hasResponse = payload.response.length > 0
  const paramCount = Object.keys(payload.params).length + (payload.model ? 1 : 0)
  const title = entry.ConversationTitle || 'Untitled conversation'

  const tabs: { id: DetailTab; label: string; count?: number }[] = [
    { id: 'conversation', label: 'Conversation', count: payload.messages.length + (hasResponse ? 1 : 0) },
    { id: 'tools', label: 'Tools', count: payload.tools.length },
    { id: 'params', label: 'Parameters', count: paramCount },
    { id: 'raw', label: 'Raw JSON' },
  ]

  return (
    <div className="rd-detail">
      <header className="rd-detail-head">
        <div className="rd-detail-heading">
          <h3 className="rd-detail-title" title={title}>{title}</h3>
          <div className="rd-detail-sub">
            {entry.Model && <span className="rd-model-chip">{entry.Model}</span>}
            <span className="rd-detail-time">{formatFull(row.createdAt)}</span>
            <span className={`rd-pill${hasResponse ? ' rd-pill--ok' : ' rd-pill--muted'}`}>
              <span className={`rd-status-dot${hasResponse ? ' rd-status-dot--ok' : ''}`} aria-hidden="true" />
              {hasResponse ? 'Reply captured' : 'No reply captured'}
            </span>
          </div>
        </div>
        <CopyButton
          text={entry.Messages}
          label="Copy full JSON"
          copiedLabel="Copied JSON"
          variant="outline"
          size="md"
          title={`Copy the full payload (${humanSize(entry.Messages.length)})`}
        />
      </header>

      <StatsStrip payload={payload} />

      <nav className="rd-tabs" role="tablist" aria-label="Request sections">
        {tabs.map(t => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            className="rd-tab"
            onClick={() => setTab(t.id)}
          >
            {t.label}
            {t.count != null && <span className="rd-tab-count">{t.count}</span>}
          </button>
        ))}
      </nav>

      <div className="rd-tab-panel" role="tabpanel" key={`${entry.ID}-${tab}`}>
        {tab === 'conversation' && <MessageTimeline payload={payload} />}
        {tab === 'tools' && <ToolDefinitionList tools={payload.tools} />}
        {tab === 'params' && <ParamsTable params={payload.params} model={payload.model} />}
        {tab === 'raw' && <RawJsonView json={entry.Messages} />}
      </div>
    </div>
  )
}
