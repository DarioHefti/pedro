import { useState, useEffect, useMemo, useCallback, useRef } from 'react'
import { useToast } from './context/ToastContext'
import { detailsService } from './services/wailsService'
import type { LLMDetailsEntry } from './services/wailsService'

/* -------------------------------------------------------------------------- */
/*  Types                                                                      */
/* -------------------------------------------------------------------------- */

interface Msg {
  role: string
  content: string
}

interface ParsedPayload {
  messages: Msg[]
  tools: string[]
  response: string
}

interface Row {
  entry: LLMDetailsEntry
  payload: ParsedPayload
  /** Cheap one-line preview for the sidebar. */
  preview: string
}

/* Hard cap on how many characters we ever hand to the DOM for a single block.
   The freeze was caused by rendering (and string-splitting) multi-MB payloads. */
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

// Tolerant field access: backend now emits lowercase role/content, but older
// stored rows used Go's default capitalized keys (Role/Content).
function pick(m: any, lower: string, upper: string): string {
  const v = m?.[lower] ?? m?.[upper]
  return v == null ? '' : String(v)
}

function toMsg(m: any): Msg {
  return { role: pick(m, 'role', 'Role'), content: pick(m, 'content', 'Content') }
}

function parse(json: string): ParsedPayload {
  const empty: ParsedPayload = { messages: [], tools: [], response: '' }
  try {
    const p = JSON.parse(json)
    if (Array.isArray(p)) {
      return { messages: p.map(toMsg), tools: [], response: '' }
    }
    const msgs = Array.isArray(p?.messages) ? p.messages : []
    const tools = Array.isArray(p?.tools) ? p.tools : []
    const response = p?.response ?? p?.Response
    return {
      messages: msgs.map(toMsg),
      tools: tools.map(String),
      response: typeof response === 'string' ? response : '',
    }
  } catch {
    return empty
  }
}

function buildPreview(payload: ParsedPayload): string {
  const lastUser = [...payload.messages].reverse().find(m => m.role === 'user')
  const source = lastUser?.content || payload.response || payload.messages[0]?.content || ''
  const flat = source.slice(0, 240).replace(/\s+/g, ' ').trim()
  return flat || 'No content'
}

function roleLabel(role: string): string {
  switch (role) {
    case 'assistant': return 'PEDRO'
    case 'tool_call': return 'TOOL CALL'
    case 'tool': return 'TOOL RESULT'
    case 'function': return 'FUNCTION'
    default: return role.toUpperCase() || 'UNKNOWN'
  }
}

function roleBadge(role: string): string {
  switch (role) {
    case 'system': return 'S'
    case 'user': return 'U'
    case 'assistant': return 'P'
    case 'tool': return 'T'
    case 'tool_call': return 'TC'
    case 'function': return 'Fn'
    default: return '?'
  }
}

function roleAccent(role: string): string {
  switch (role) {
    case 'system': return 'tl--system'
    case 'user': return 'tl--user'
    case 'assistant': return 'tl--assistant'
    case 'tool': return 'tl--tool'
    case 'tool_call':
    case 'function': return 'tl--toolcall'
    default: return 'tl--system'
  }
}

/* -------------------------------------------------------------------------- */
/*  Timeline node (a single message)                                          */
/* -------------------------------------------------------------------------- */

function TimelineNode({ msg, isLast }: { msg: Msg; isLast: boolean }) {
  const [expanded, setExpanded] = useState(false)
  const [copied, setCopied] = useState(false)

  const content = msg.content || ''
  const collapsible = content.length > 600
  const truncated = content.length > MAX_RENDER

  // Cheap preview: only ever split a small bounded slice — never the whole payload.
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
          <span className="hd-chip">{payload.tools.length} tools available</span>
        )}
        <span className={`hd-chip ${hasResponse ? 'hd-chip--ok' : 'hd-chip--muted'}`}>
          {hasResponse ? 'has reply' : 'no reply captured'}
        </span>
      </div>

      {payload.tools.length > 0 && (
        <div className="hd-tools">
          {payload.tools.map((t, i) => (
            <span key={i} className="hd-tool">{t}</span>
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
}

export default function Details({ open, onClose }: DetailsProps) {
  const toast = useToast()
  // Keep a stable ref so `load` never changes identity (prevents effect churn).
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

  // Escape closes the modal.
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  // Parse every entry exactly once (not on every keystroke / render).
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
      r.payload.messages.some(m => m.role.toLowerCase().includes(q) || m.content.toLowerCase().includes(q)),
    )
  }, [rows, filter])

  // Keep a valid selection as data / filters change.
  useEffect(() => {
    if (filtered.length === 0) {
      setSelectedId(null)
      return
    }
    setSelectedId(prev =>
      prev != null && filtered.some(r => r.entry.ID === prev) ? prev : filtered[0].entry.ID,
    )
  }, [filtered])

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
