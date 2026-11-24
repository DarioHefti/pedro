import { useState, type ReactNode } from 'react'
import { Braces, MessagesSquare, SlidersHorizontal, Wrench } from 'lucide-react'
import { Tabs, TabsIndicator, TabsList, TabsPanel, TabsTab } from '@ui-sdk/react/tabs'
import { CopyButton } from './DetailsButton'
import { MessageTimeline } from './MessageTimeline'
import { ParamsTable } from './ParamsTable'
import { RawJsonView } from './RawJsonView'
import { ToolDefinitionList } from './ToolDefinitionList'
import { ROLE_ORDER, compactNumber, formatFull, humanSize, roleLabel } from './payload'
import type { ParsedPayload, RequestRow } from './types'

type DetailTab = 'conversation' | 'tools' | 'params' | 'raw'

function RoleBreakdownBar({ payload }: { payload: ParsedPayload }) {
  if (payload.messages.length === 0) return null
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

function StatCard({ label, value, hint, children }: { label: string; value: string; hint?: string; children?: ReactNode }) {
  return (
    <div className="rd-stat">
      <span className="rd-stat-label">{label}</span>
      <span className="rd-stat-value">{value}</span>
      {hint ? <span className="rd-stat-hint">{hint}</span> : null}
      {children}
    </div>
  )
}

function StatsStrip({ payload }: { payload: ParsedPayload }) {
  const breakdown = ROLE_ORDER.filter(r => payload.roleCounts[r] > 0)
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

export function RequestDetailView({ row }: { row: RequestRow }) {
  const { entry, payload } = row
  const [tab, setTab] = useState<DetailTab>('conversation')
  const hasResponse = payload.response.length > 0
  const paramCount = Object.keys(payload.params).length + (payload.model ? 1 : 0)
  const title = entry.ConversationTitle || 'Untitled conversation'

  const tabs: { id: DetailTab; label: string; icon: typeof Braces; count?: number }[] = [
    { id: 'conversation', label: 'Conversation', icon: MessagesSquare, count: payload.messages.length + (hasResponse ? 1 : 0) },
    { id: 'tools', label: 'Tools', icon: Wrench, count: payload.tools.length },
    { id: 'params', label: 'Parameters', icon: SlidersHorizontal, count: paramCount },
    { id: 'raw', label: 'Raw JSON', icon: Braces },
  ]

  return (
    <div className="rd-detail">
      <header className="rd-detail-head">
        <div className="rd-detail-heading">
          <h3 className="rd-detail-title" title={title}>
            {title}
          </h3>
          <div className="rd-detail-sub">
            {entry.Model ? <span data-badge="brand">{entry.Model}</span> : null}
            <span className="rd-detail-time">{formatFull(row.createdAt)}</span>
            <span data-badge={hasResponse ? 'success' : 'neutral'}>
              <span className="status-dot" data-tone={hasResponse ? 'success' : undefined} aria-hidden="true" />
              {hasResponse ? 'Reply captured' : 'No reply captured'}
            </span>
          </div>
        </div>
        <CopyButton
          text={entry.Messages}
          label="Copy full JSON"
          copiedLabel="Copied JSON"
          variant="secondary"
          size="sm"
          title={`Copy the full payload (${humanSize(entry.Messages.length)})`}
        />
      </header>

      <StatsStrip payload={payload} />

      <Tabs<DetailTab> value={tab} onValueChange={v => v && setTab(v)} className="rd-tabs-root">
        <TabsList data-variant="underline" aria-label="Request sections" className="rd-tabs">
          <TabsIndicator />
          {tabs.map(t => {
            const Icon = t.icon
            return (
              <TabsTab key={t.id} value={t.id}>
                <Icon aria-hidden="true" />
                {t.label}
                {t.count != null ? <span className="tab-count">{t.count}</span> : null}
              </TabsTab>
            )
          })}
        </TabsList>
        <TabsPanel value="conversation" className="rd-tab-panel">
          <MessageTimeline payload={payload} />
        </TabsPanel>
        <TabsPanel value="tools" className="rd-tab-panel">
          <ToolDefinitionList tools={payload.tools} />
        </TabsPanel>
        <TabsPanel value="params" className="rd-tab-panel">
          <ParamsTable params={payload.params} model={payload.model} />
        </TabsPanel>
        <TabsPanel value="raw" className="rd-tab-panel">
          <RawJsonView json={entry.Messages} />
        </TabsPanel>
      </Tabs>
    </div>
  )
}
