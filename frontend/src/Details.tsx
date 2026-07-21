import { useState, useEffect, useMemo, useCallback, useRef } from 'react'
import { useToast } from './context/ToastContext'
import { detailsService } from './services/wailsService'
import type { LLMDetailsEntry } from './services/wailsService'

/* -------------------------------------------------------------------------- */
/*  Types                                                                      */
/* -------------------------------------------------------------------------- */

// OpenAI API message structures (directly from the request JSON)

interface OAIPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: { url: string; detail?: string }
}

interface OAIToolCall {
  id: string
  type: 'function'
  function: { name: string; arguments: string }
}

interface OAIMessage {
  role: 'system' | 'user' | 'assistant' | 'tool'
  content: string | OAIPart[] | null
  tool_calls?: OAIToolCall[]
  tool_call_id?: string
}

interface OAIToolDef {
  type: 'function'
  function: {
    name: string
    description?: string
    parameters?: {
      type: string
      properties?: Record<string, unknown>
      required?: string[]
    }
  }
}

// Parsed payload for the UI

interface ParsedPayload {
  messages: OAIMessage[]
  tools: OAIToolDef[]
  model: string
  params: Record<string, unknown>
  response: string
}

interface Row {
  entry: LLMDetailsEntry
  payload: ParsedPayload
  preview: string
}

/* Hard cap on how many characters we ever hand to the DOM for a single block. */
const MAX_RENDER = 24000
/* Only scan this many chars when building a collapsed preview. */
const PREVIEW_SCAN = 1600

/* -------------------------------------------------------------------------- */
/*  Helpers                                                                    */
/* -------------------------------------------------------------------------- */

function fmt(raw: unknown): string {
  const value = typeof raw === 'string' ? raw : String(raw ?? '')
  try {
    const d = new Date(value)
    if (Number.isNaN(d.getTime())) return value
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }).format(d)
  } catch {
    return value
  }
}

function humanSize(len: number): string {
  if (len < 1000) return `${len} chars`
  if (len < 1_000_000) return `${(len / 1000).toFixed(len < 10_000 ? 1 : 0)}k chars`
  return `${(len / 1_000_000).toFixed(1)}M chars`
}

/**
 * Normalize any OpenAI content shape to a plain string.
 */
function normalizeContent(c: string | OAIPart[] | null | undefined): string {
  if (c == null) return ''
  if (typeof c === 'string') return c
  return c
    .map(p => {
      if (p.type === 'text') return p.text ?? ''
      if (p.type === 'image_url') return `[image: ${p.image_url?.url ?? ''}]`
      return ''
    })
    .filter(Boolean)
    .join('\n')
}

/**
 * Extract interesting request parameters (non-default, non-message fields).
 */
function extractParams(request: Record<string, unknown>): Record<string, unknown> {
  const interesting = [
    'temperature', 'max_tokens', 'max_completion_tokens', 'top_p',
    'frequency_penalty', 'presence_penalty', 'seed', 'n', 'stop',
    'response_format', 'tool_choice', 'parallel_tool_calls',
    'reasoning_effort', 'service_tier',
  ]
  const params: Record<string, unknown> = {}
  for (const key of interesting) {
    if (request[key] !== undefined && request[key] !== null) {
      params[key] = request[key]
    }
  }
  return params
}

/**
 * Pretty-print tool call arguments (parse JSON, re-serialize with indent).
 */
function formatToolArgs(argsJson: string): string {
  try {
    return JSON.stringify(JSON.parse(argsJson), null, 2)
  } catch {
    return argsJson
  }
}

function parse(json: string): ParsedPayload {
  const empty: ParsedPayload = { messages: [], tools: [], model: '', params: {}, response: '' }
  try {
    const p = JSON.parse(json)
    const req = p.request
    if (!req || typeof req !== 'object') return empty

    return {
      messages: Array.isArray(req.messages) ? req.messages : [],
      tools: Array.isArray(req.tools) ? req.tools : [],
      model: typeof req.model === 'string' ? req.model : '',
      params: extractParams(req),
      response: typeof p.response === 'string' ? p.response : '',
    }
  } catch {
    return empty
  }
}

function buildPreview(payload: ParsedPayload): string {
  const lastUser = [...payload.messages].reverse().find(m => m.role === 'user')
  const content = lastUser ? normalizeContent(lastUser.content) : ''
  const source = content || payload.response || normalizeContent(payload.messages[0]?.content) || ''
  const flat = source.slice(0, 240).replace(/\s+/g, ' ').trim()
  return flat || 'No content'
}

function roleLabel(role: string): string {
  switch (role) {
    case 'assistant': return 'PEDRO'
    case 'tool': return 'TOOL RESULT'
    default: return role.toUpperCase() || 'UNKNOWN'
  }
}

function roleBadge(role: string): string {
  switch (role) {
    case 'system': return 'S'
    case 'user': return 'U'
    case 'assistant': return 'P'
    case 'tool': return 'T'
    default: return '?'
  }
}

function roleAccent(role: string): string {
  switch (role) {
    case 'system': return 'tl--system'
    case 'user': return 'tl--user'
    case 'assistant': return 'tl--assistant'
    case 'tool': return 'tl--tool'
    default: return 'tl--system'
  }
}

/* -------------------------------------------------------------------------- */
/*  Timeline node (a single message)                                          */
/* -------------------------------------------------------------------------- */

function TimelineNode({ msg, isLast }: { msg: OAIMessage; isLast: boolean }) {
  const [expanded, setExpanded] = useState(false)
  const [copied, setCopied] = useState(false)

  const content = normalizeContent(msg.content)
  const collapsible = content.length > 600
  const truncated = content.length > MAX_RENDER

  const previewLines = useMemo(() => {
    const slice = content.length > PREVIEW_SCAN ? content.slice(0, PREVIEW_SCAN) : content
    return slice.split('\n').slice(0, 3)
  }, [content])

  const shown = truncated ? content.slice(0, MAX_RENDER) : content

  async function copy() {
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1400)
    } catch { /* ignore */ }
  }

  const open = expanded || !collapsible

  return (
    <div className={`tl ${roleAccent(msg.role)}${isLast ? ' tl--last' : ''}`}>
      <div className="tl-rail">
        <span className="tl-dot" />
        {!isLast && <span className="tl-line" />}
      </div>

      <div className="tl-card">
        <div className="tl-head">
          <span className="tl-badge">{roleBadge(msg.role)}</span>
          <span className="tl-role">{roleLabel(msg.role)}</span>
          {msg.role === 'tool' && msg.tool_call_id && (
            <span className="tl-tag">call: {msg.tool_call_id}</span>
          )}
          <span className="tl-size">{humanSize(content.length)}</span>
          <button
            type="button"
            className="tl-copy"
            title="Copy this message"
            onClick={copy}
          >
            {copied ? 'Copied' : 'Copy'}
          </button>
          {collapsible && (
            <button
              type="button"
              className="tl-toggle"
              onClick={() => setExpanded(e => !e)}
            >
              {expanded ? 'Collapse' : 'Expand'}
            </button>
          )}
        </div>

        {open ? (
          <>
            <pre className="tl-body">{shown}</pre>
            {truncated && (
              <div className="tl-trunc">
                Showing first {humanSize(MAX_RENDER)} — use Copy to grab the full message.
              </div>
            )}
          </>
        ) : (
          <div className="tl-preview" onClick={() => setExpanded(true)} role="button" tabIndex={0}>
            {previewLines.map((l, i) => (
              <span key={i} className="tl-preview-line">{l || '\u00A0'}</span>
            ))}
            <span className="tl-preview-more">Click to expand</span>
          </div>
        )}

        {msg.tool_calls && msg.tool_calls.length > 0 && (
          <div className="tl-toolcalls">
            {msg.tool_calls.map(tc => (
              <div key={tc.id} className="tl-toolcall">
                <div className="tl-toolcall-head">
                  <span className="tl-toolcall-name">{tc.function.name}</span>
                  <span className="tl-toolcall-id">{tc.id}</span>
                </div>
                <pre className="tl-toolcall-args">{formatToolArgs(tc.function.arguments)}</pre>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function ResponseNode({ content }: { content: string }) {
  const [copied, setCopied] = useState(false)
  if (!content) return null

  const truncated = content.length > MAX_RENDER
  const shown = truncated ? content.slice(0, MAX_RENDER) : content

  async function copy() {
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1400)
    } catch { /* ignore */ }
  }

  return (
    <div className="tl tl--assistant tl--response tl--last">
      <div className="tl-rail">
        <span className="tl-dot tl-dot--glow" />
      </div>
      <div className="tl-card tl-card--response">
        <div className="tl-head">
          <span className="tl-badge tl-badge--response">P</span>
          <span className="tl-role">PEDRO</span>
          <span className="tl-tag">response</span>
          <span className="tl-size">{humanSize(content.length)}</span>
          <button type="button" className="tl-copy" onClick={copy}>
            {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
        <pre className="tl-body tl-body--response">{shown}</pre>
        {truncated && (
          <div className="tl-trunc">
            Showing first {humanSize(MAX_RENDER)} — use Copy to grab the full response.
          </div>
        )}
      </div>
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/*  Tool definition card                                                      */
/* -------------------------------------------------------------------------- */

function ToolDefCard({ tool }: { tool: OAIToolDef }) {
  const [expanded, setExpanded] = useState(false)
  const name = tool.function?.name ?? 'unknown'
  const desc = tool.function?.description
  const params = tool.function?.parameters

  return (
    <div className="tooldef">
      <div className="tooldef-head" onClick={() => setExpanded(e => !e)}>
        <span className="tooldef-name">{name}</span>
        {desc && <span className="tooldef-desc">{desc}</span>}
        {params && (
          <span className="tooldef-toggle">{expanded ? '\u25BE' : '\u25B8'} params</span>
        )}
      </div>
      {expanded && params && (
        <pre className="tooldef-params">{JSON.stringify(params, null, 2)}</pre>
      )}
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/*  Raw JSON view                                                             */
/* -------------------------------------------------------------------------- */

function RawJsonView({ json }: { json: string }) {
  const [open, setOpen] = useState(false)
  const [copied, setCopied] = useState(false)

  const pretty = useMemo(() => {
    try { return JSON.stringify(JSON.parse(json), null, 2) }
    catch { return json }
  }, [json])

  async function copy() {
    await navigator.clipboard.writeText(pretty)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="rawjson">
      <div className="rawjson-bar">
        <button type="button" className="rawjson-toggle" onClick={() => setOpen(o => !o)}>
          {open ? 'Hide' : 'Show'} Raw JSON
        </button>
        {open && (
          <button type="button" className="tl-copy" onClick={copy}>
            {copied ? 'Copied' : 'Copy'}
          </button>
        )}
      </div>
      {open && <pre className="rawjson-body">{pretty}</pre>}
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/*  Detail pane (one selected request)                                        */
/* -------------------------------------------------------------------------- */

function DetailView({ row }: { row: Row }) {
  const { entry, payload } = row
  const [copiedAll, setCopiedAll] = useState(false)

  async function copyAll() {
    try {
      await navigator.clipboard.writeText(entry.Messages)
      setCopiedAll(true)
      window.setTimeout(() => setCopiedAll(false), 1500)
    } catch { /* ignore */ }
  }

  const hasResponse = payload.response.length > 0

  return (
    <div className="hd">
      <div className="hd-top">
        <div className="hd-title">
          <span className="hd-conv">{entry.ConversationTitle || 'Untitled'}</span>
          <span className="hd-model">{entry.Model}</span>
        </div>
        <button type="button" className="hd-copyall" onClick={copyAll}>
          {copiedAll ? 'Copied JSON' : 'Copy full JSON'}
        </button>
      </div>

      <div className="hd-meta">
        <span className="hd-chip">{fmt(entry.CreatedAt)}</span>
        <span className="hd-chip">{payload.messages.length} messages</span>
        {payload.tools.length > 0 && (
          <span className="hd-chip">{payload.tools.length} tools</span>
        )}
        {Object.keys(payload.params).length > 0 && (
          <span className="hd-chip">{Object.keys(payload.params).length} params</span>
        )}
        <span className={`hd-chip ${hasResponse ? 'hd-chip--ok' : 'hd-chip--muted'}`}>
          {hasResponse ? 'has reply' : 'no reply captured'}
        </span>
      </div>

      {Object.keys(payload.params).length > 0 && (
        <div className="hd-params">
          {Object.entries(payload.params).map(([k, v]) => (
            <span key={k} className="hd-chip hd-chip--param">
              {k}: {JSON.stringify(v)}
            </span>
          ))}
        </div>
      )}

      {payload.tools.length > 0 && (
        <div className="hd-tooldefs">
          {payload.tools.map((t, i) => (
            <ToolDefCard key={i} tool={t} />
          ))}
        </div>
      )}

      <div className="hd-timeline">
        {payload.messages.length === 0 && !hasResponse ? (
          <div className="hd-empty">This request had no readable payload.</div>
        ) : (
          <>
            {payload.messages.map((m, i) => (
              <TimelineNode
                key={i}
                msg={m}
                isLast={!hasResponse && i === payload.messages.length - 1}
              />
            ))}
            <ResponseNode content={payload.response} />
          </>
        )}
      </div>

      <RawJsonView json={entry.Messages} />
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/*  Sidebar row                                                                */
/* -------------------------------------------------------------------------- */

function RequestRow({
  row,
  active,
  onSelect,
}: {
  row: Row
  active: boolean
  onSelect: () => void
}) {
  const { entry, payload } = row
  return (
    <button
      type="button"
      className={`hr${active ? ' hr--active' : ''}`}
      onClick={onSelect}
    >
      <span className="hr-indicator" />
      <span className="hr-top">
        <span className="hr-conv">{entry.ConversationTitle || 'Untitled'}</span>
        <span className="hr-time">{fmt(entry.CreatedAt)}</span>
      </span>
      {entry.Model && <span className="hr-model">{entry.Model}</span>}
      <span className="hr-preview">{row.preview}</span>
      <span className="hr-stats">
        <span className="hr-stat">{payload.messages.length} msgs</span>
        {payload.tools.length > 0 && <span className="hr-stat">{payload.tools.length} tools</span>}
        {payload.response && <span className="hr-stat hr-stat--ok">reply</span>}
      </span>
    </button>
  )
}

/* -------------------------------------------------------------------------- */
/*  Main modal                                                                 */
/* -------------------------------------------------------------------------- */

interface DetailsProps {
  open: boolean
  onClose: () => void
  /** When provided, auto-select the detail entry for this conversation on open. */
  conversationID?: number | null
}

export default function Details({ open, onClose, conversationID }: DetailsProps) {
  const toast = useToast()
  const toastRef = useRef(toast)
  toastRef.current = toast

  const [entries, setEntries] = useState<LLMDetailsEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [filter, setFilter] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [confirmingClear, setConfirmingClear] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const all = await detailsService.getAll()
      setEntries(all ?? [])
    } catch (err) {
      toastRef.current.error('Failed to load Request Details: ' + String(err))
      setEntries([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) {
      setConfirmingClear(false)
      void load()
    }
  }, [open, load])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  const rows = useMemo<Row[]>(() => {
    return entries.map(entry => {
      const payload = parse(entry.Messages)
      return { entry, payload, preview: buildPreview(payload) }
    })
  }, [entries])

  const filtered = useMemo<Row[]>(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return rows
    return rows.filter(r =>
      (r.entry.Model || '').toLowerCase().includes(q) ||
      (r.entry.ConversationTitle || '').toLowerCase().includes(q) ||
      r.preview.toLowerCase().includes(q) ||
      r.payload.response.toLowerCase().includes(q) ||
      r.payload.messages.some(m => {
        const role = m.role?.toLowerCase() ?? ''
        const content = normalizeContent(m.content).toLowerCase()
        return role.includes(q) || content.includes(q)
      }),
    )
  }, [rows, filter])

  useEffect(() => {
    if (filtered.length === 0) {
      setSelectedId(null)
      return
    }
    setSelectedId(prev => {
      // If a specific conversation is requested, prefer its entry
      if (conversationID != null) {
        const match = filtered.find(r => r.entry.ConversationID === conversationID)
        if (match) return match.entry.ID
      }
      // Keep current selection if still valid
      if (prev != null && filtered.some(r => r.entry.ID === prev)) return prev
      return filtered[0].entry.ID
    })
  }, [filtered, conversationID])

  const selected = useMemo(
    () => filtered.find(r => r.entry.ID === selectedId) ?? null,
    [filtered, selectedId],
  )

  async function clearAll() {
    try {
      await detailsService.clear()
      setEntries([])
      setSelectedId(null)
      setConfirmingClear(false)
      toast.success('Request Details cleared')
    } catch (err) {
      toast.error('Failed to clear details: ' + String(err))
    }
  }

  if (!open) return null

  return (
    <div className="modal details-overlay" onClick={onClose}>
      <div
        className="modal-content details-modal"
        onClick={e => e.stopPropagation()}
        role="dialog"
        aria-label="Request Details"
      >
        {/* Header */}
        <header className="hm-head">
          <div className="hm-head-text">
            <h2>Request Details</h2>
            <p className="hm-sub">The last payload Pedro sent to the LLM for each conversation.</p>
          </div>
          <div className="hm-head-actions">
            {confirmingClear ? (
              <div className="hm-confirm">
                <span className="hm-confirm-text">Delete all {entries.length}?</span>
                <button type="button" className="hm-confirm-no" onClick={() => setConfirmingClear(false)}>
                  Cancel
                </button>
                <button type="button" className="hm-confirm-yes" onClick={() => void clearAll()}>
                  Clear all
                </button>
              </div>
            ) : (
              <button
                type="button"
                className="hm-clear"
                onClick={() => setConfirmingClear(true)}
                disabled={entries.length === 0}
              >
                Clear all
              </button>
            )}
            <button type="button" className="hm-close" onClick={onClose} aria-label="Close">
              ✕
            </button>
          </div>
        </header>

        {/* Body: master / detail */}
        <div className="hm-body">
          <aside className="hm-list-pane">
            <div className="hm-search">
              <input
                type="text"
                className="hm-search-input"
                placeholder="Search model, role, content…"
                value={filter}
                onChange={e => setFilter(e.target.value)}
              />
              <span className="hm-count">{filtered.length}/{entries.length}</span>
            </div>

            <div className="hm-list">
              {loading ? (
                <div className="hm-skeletons">
                  {Array.from({ length: 5 }).map((_, i) => (
                    <div key={i} className="hm-skel" />
                  ))}
                </div>
              ) : filtered.length === 0 ? (
                <div className="hm-list-empty">
                  {entries.length === 0 ? 'No requests recorded yet' : 'No matches'}
                </div>
              ) : (
                filtered.map(r => (
                  <RequestRow
                    key={r.entry.ID}
                    row={r}
                    active={r.entry.ID === selectedId}
                    onSelect={() => setSelectedId(r.entry.ID)}
                  />
                ))
              )}
            </div>
          </aside>

          <section className="hm-detail-pane">
            {loading ? (
              <div className="hm-detail-empty">
                <div className="hm-detail-empty-title">Loading…</div>
              </div>
            ) : selected ? (
              <DetailView key={selected.entry.ID} row={selected} />
            ) : (
              <div className="hm-detail-empty">
                <div className="hm-detail-empty-title">
                  {entries.length === 0 ? 'Nothing here yet' : 'Select a request'}
                </div>
                <div className="hm-detail-empty-sub">
                  {entries.length === 0
                    ? 'Send a message to Pedro and every payload will show up here.'
                    : 'Pick a request on the left to inspect the full timeline.'}
                </div>
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  )
}
