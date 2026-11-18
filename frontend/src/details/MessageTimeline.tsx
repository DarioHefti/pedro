import { useMemo, useState } from 'react'
import { CopyButton, DetailsButton } from './DetailsButton'
import {
  COLLAPSE_THRESHOLD,
  MAX_RENDER,
  PREVIEW_SCAN,
  ROLE_ORDER,
  humanSize,
  normalizeContent,
  prettyJson,
  roleInitial,
  roleLabel,
} from './payload'
import type { OAIMessage, OAIRole, OAIToolCall, ParsedPayload } from './types'

/* -------------------------------------------------------------------------- */
/*  Message body (shared by request messages and the response)                */
/* -------------------------------------------------------------------------- */

function MessageBody({
  content,
  open,
  onExpand,
}: {
  content: string
  open: boolean
  onExpand: () => void
}) {
  const previewLines = useMemo(() => {
    const slice = content.length > PREVIEW_SCAN ? content.slice(0, PREVIEW_SCAN) : content
    return slice.split('\n').slice(0, 3)
  }, [content])

  if (!content) return <div className="rd-msg-empty">No text content</div>

  if (!open) {
    return (
      <button type="button" className="rd-msg-preview" onClick={onExpand}>
        {previewLines.map((l, i) => (
          <span key={i} className="rd-msg-preview-line">{l || ' '}</span>
        ))}
        <span className="rd-msg-preview-more">Show all {humanSize(content.length)}</span>
      </button>
    )
  }

  const truncated = content.length > MAX_RENDER
  return (
    <>
      <pre className="rd-msg-body">{truncated ? content.slice(0, MAX_RENDER) : content}</pre>
      {truncated && (
        <div className="rd-msg-trunc">
          Showing first {humanSize(MAX_RENDER)} — use Copy to grab the full message.
        </div>
      )}
    </>
  )
}

/* -------------------------------------------------------------------------- */
/*  Tool calls requested by the assistant                                     */
/* -------------------------------------------------------------------------- */

function ToolCallList({ calls }: { calls: OAIToolCall[] }) {
  return (
    <div className="rd-toolcalls">
      {calls.map(tc => (
        <details key={tc.id} className="rd-toolcall">
          <summary className="rd-toolcall-head">
            <span className="rd-toolcall-chevron" aria-hidden="true">▸</span>
            <span className="rd-toolcall-name">{tc.function.name}</span>
            <span className="rd-toolcall-id">{tc.id}</span>
          </summary>
          <pre className="rd-code rd-code--compact">{prettyJson(tc.function.arguments)}</pre>
        </details>
      ))}
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/*  One timeline item                                                          */
/* -------------------------------------------------------------------------- */

interface TimelineItemProps {
  role: OAIRole | 'response'
  index?: number
  content: string
  tag?: string
  toolCalls?: OAIToolCall[]
  expanded: boolean
  onToggle: () => void
}

function TimelineItem({ role, index, content, tag, toolCalls, expanded, onToggle }: TimelineItemProps) {
  const isResponse = role === 'response'
  const collapsible = !isResponse && content.length > COLLAPSE_THRESHOLD
  const open = expanded || !collapsible

  return (
    <article className={`rd-msg rd-msg--${role}`}>
      <div className="rd-msg-rail" aria-hidden="true">
        <span className="rd-msg-avatar">{roleInitial(isResponse ? 'assistant' : role)}</span>
      </div>

      <div className="rd-msg-card">
        <header className="rd-msg-head">
          <span className="rd-msg-role">{isResponse ? 'Pedro' : roleLabel(role)}</span>
          {isResponse && <span className="rd-tag rd-tag--accent">response</span>}
          {tag && <span className="rd-tag" title={tag}>{tag}</span>}
          {toolCalls && toolCalls.length > 0 && (
            <span className="rd-tag rd-tag--tool">
              {toolCalls.length} tool call{toolCalls.length === 1 ? '' : 's'}
            </span>
          )}
          <span className="rd-msg-size">
            {index != null && <span className="rd-msg-index">#{index + 1}</span>}
            {humanSize(content.length)}
          </span>
          <span className="rd-msg-actions">
            {content && <CopyButton text={content} title="Copy this message" />}
            {collapsible && (
              <DetailsButton onClick={onToggle}>{expanded ? 'Collapse' : 'Expand'}</DetailsButton>
            )}
          </span>
        </header>

        <MessageBody content={content} open={open} onExpand={onToggle} />
        {toolCalls && toolCalls.length > 0 && <ToolCallList calls={toolCalls} />}
      </div>
    </article>
  )
}

/* -------------------------------------------------------------------------- */
/*  Timeline                                                                   */
/* -------------------------------------------------------------------------- */

type RoleFilter = OAIRole | 'all'

/** Map tool_call ids to the function name the assistant invoked. */
function toolNamesById(messages: OAIMessage[]): Map<string, string> {
  const map = new Map<string, string>()
  for (const m of messages) {
    for (const tc of m.tool_calls ?? []) map.set(tc.id, tc.function.name)
  }
  return map
}

export function MessageTimeline({ payload }: { payload: ParsedPayload }) {
  const { messages, response, roleCounts } = payload
  const [roleFilter, setRoleFilter] = useState<RoleFilter>('all')
  const [expanded, setExpanded] = useState<Set<number>>(() => new Set())

  const contents = useMemo(() => messages.map(m => normalizeContent(m.content)), [messages])
  const toolNames = useMemo(() => toolNamesById(messages), [messages])
  const collapsibleIdx = useMemo(
    () => contents.flatMap((c, i) => (c.length > COLLAPSE_THRESHOLD ? [i] : [])),
    [contents],
  )
  const allExpanded = collapsibleIdx.length > 0 && collapsibleIdx.every(i => expanded.has(i))

  function toggle(i: number) {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(i)) next.delete(i)
      else next.add(i)
      return next
    })
  }

  function toggleAll() {
    setExpanded(allExpanded ? new Set() : new Set(collapsibleIdx))
  }

  const visible = messages
    .map((m, i) => ({ m, i }))
    .filter(({ m }) => roleFilter === 'all' || m.role === roleFilter)
  const showResponse = response.length > 0 && (roleFilter === 'all' || roleFilter === 'assistant')

  if (messages.length === 0 && !response) {
    return <div className="rd-empty-block">This request had no readable messages.</div>
  }

  return (
    <div className="rd-timeline-wrap">
      <div className="rd-toolbar">
        <div className="rd-segmented" role="tablist" aria-label="Filter by role">
          <button
            type="button"
            role="tab"
            aria-selected={roleFilter === 'all'}
            className="rd-segment"
            onClick={() => setRoleFilter('all')}
          >
            All <span className="rd-segment-count">{messages.length}</span>
          </button>
          {ROLE_ORDER.filter(r => roleCounts[r] > 0).map(r => (
            <button
              key={r}
              type="button"
              role="tab"
              aria-selected={roleFilter === r}
              className={`rd-segment rd-segment--${r}`}
              onClick={() => setRoleFilter(r)}
            >
              <span className="rd-segment-dot" aria-hidden="true" />
              {roleLabel(r)} <span className="rd-segment-count">{roleCounts[r]}</span>
            </button>
          ))}
        </div>
        {collapsibleIdx.length > 0 && (
          <DetailsButton className="rd-toolbar-end" onClick={toggleAll}>{allExpanded ? 'Collapse all' : 'Expand all'}</DetailsButton>
        )}
      </div>

      <div className="rd-timeline">
        {visible.map(({ m, i }) => (
          <TimelineItem
            key={i}
            role={m.role}
            index={i}
            content={contents[i]}
            tag={m.role === 'tool' && m.tool_call_id ? toolNames.get(m.tool_call_id) ?? m.tool_call_id : undefined}
            toolCalls={m.tool_calls}
            expanded={expanded.has(i)}
            onToggle={() => toggle(i)}
          />
        ))}
        {showResponse && (
          <TimelineItem role="response" content={response} expanded onToggle={() => {}} />
        )}
        {visible.length === 0 && !showResponse && (
          <div className="rd-empty-block">No {roleLabel(roleFilter).toLowerCase()} messages.</div>
        )}
      </div>
    </div>
  )
}
