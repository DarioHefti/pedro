import { useState, useEffect, useMemo, useCallback, useRef } from 'react'
import { useToast } from './context/ToastContext'
import { detailsService } from './services/wailsService'
import type { LLMDetailsEntry } from './services/wailsService'
import { DetailsButton } from './details/DetailsButton'
import { RequestDetailView } from './details/RequestDetailView'
import { RequestList } from './details/RequestList'
import { buildRow } from './details/payload'
import type { RequestRow } from './details/types'

/* -------------------------------------------------------------------------- */
/*  Header actions                                                             */
/* -------------------------------------------------------------------------- */

function ClearAllControl({ count, onConfirm }: { count: number; onConfirm: () => void }) {
  const [confirming, setConfirming] = useState(false)

  if (!confirming) {
    return (
      <DetailsButton variant="danger" size="md" onClick={() => setConfirming(true)} disabled={count === 0}>
        Clear all
      </DetailsButton>
    )
  }

  return (
    <div className="rd-confirm" role="alertdialog" aria-label="Confirm clearing all requests">
      <span className="rd-confirm-text">Delete all {count}?</span>
      <DetailsButton variant="outline" onClick={() => setConfirming(false)}>Cancel</DetailsButton>
      <DetailsButton
        variant="danger-solid"
        onClick={() => {
          setConfirming(false)
          onConfirm()
        }}
      >
        Clear all
      </DetailsButton>
    </div>
  )
}

function EmptyDetail({ hasEntries }: { hasEntries: boolean }) {
  return (
    <div className="rd-detail-empty">
      <div className="rd-detail-empty-icon" aria-hidden="true">{'{ }'}</div>
      <div className="rd-detail-empty-title">{hasEntries ? 'Select a request' : 'Nothing here yet'}</div>
      <div className="rd-detail-empty-sub">
        {hasEntries
          ? 'Pick a request on the left to inspect the full payload.'
          : 'Send a message to Pedro and every payload will show up here.'}
      </div>
    </div>
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

  const detailPaneRef = useRef<HTMLElement>(null)
  const [entries, setEntries] = useState<LLMDetailsEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [filter, setFilter] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)

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
    if (open) void load()
  }, [open, load])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  const rows = useMemo<RequestRow[]>(() => entries.map(buildRow), [entries])

  const filtered = useMemo<RequestRow[]>(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return rows
    return rows.filter(r => r.searchText.includes(q))
  }, [rows, filter])

  useEffect(() => {
    if (filtered.length === 0) {
      setSelectedId(null)
      return
    }
    setSelectedId(prev => {
      // Keep current selection if still visible
      if (prev != null && filtered.some(r => r.entry.ID === prev)) return prev
      // Otherwise prefer the entry of the conversation the modal was opened from
      if (conversationID != null) {
        const match = filtered.find(r => r.entry.ConversationID === conversationID)
        if (match) return match.entry.ID
      }
      return filtered[0].entry.ID
    })
  }, [filtered, conversationID])

  // Re-select the current conversation's entry each time the modal opens.
  useEffect(() => {
    if (open) setSelectedId(null)
  }, [open])

  // Start each newly selected request at the top.
  useEffect(() => {
    detailPaneRef.current?.scrollTo({ top: 0 })
  }, [selectedId])

  const selected = useMemo(
    () => filtered.find(r => r.entry.ID === selectedId) ?? null,
    [filtered, selectedId],
  )

  async function clearAll() {
    try {
      await detailsService.clear()
      setEntries([])
      setSelectedId(null)
      toast.success('Request Details cleared')
    } catch (err) {
      toast.error('Failed to clear details: ' + String(err))
    }
  }

  if (!open) return null

  return (
    <div className="modal rd-overlay" onClick={onClose}>
      <div
        className="modal-content rd-modal"
        onClick={e => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Request Details"
      >
        <header className="rd-head">
          <div className="rd-head-text">
            <h2 className="rd-head-title">Request Details</h2>
            <p className="rd-head-sub">The last payload Pedro sent to the LLM for each conversation.</p>
          </div>
          <div className="rd-head-actions">
            <ClearAllControl count={entries.length} onConfirm={() => void clearAll()} />
            <DetailsButton variant="icon" size="md" onClick={onClose} aria-label="Close">
              ✕
            </DetailsButton>
          </div>
        </header>

        <div className="rd-body">
          <RequestList
            rows={filtered}
            totalCount={entries.length}
            loading={loading}
            filter={filter}
            onFilterChange={setFilter}
            selectedId={selectedId}
            onSelect={setSelectedId}
          />

          <section className="rd-detail-pane" ref={detailPaneRef}>
            {loading ? (
              <div className="rd-detail-empty">
                <div className="rd-detail-empty-title">Loading…</div>
              </div>
            ) : selected ? (
              <RequestDetailView row={selected} />
            ) : (
              <EmptyDetail hasEntries={entries.length > 0} />
            )}
          </section>
        </div>
      </div>
    </div>
  )
}
