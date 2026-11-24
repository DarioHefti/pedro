import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Activity, Braces, Trash2 } from 'lucide-react'
import { detailsService } from './services/wailsService'
import type { LLMDetailsEntry } from './services/wailsService'
import { RequestDetailView } from './details/RequestDetailView'
import { RequestList } from './details/RequestList'
import { buildRow } from './details/payload'
import type { RequestRow } from './details/types'
import { Button } from './ui/button'
import { confirmAction } from './ui/confirm'
import { AppDialog } from './ui/dialog'
import { toast } from './ui/toaster'

function EmptyDetail({ hasEntries }: { hasEntries: boolean }) {
  return (
    <div className="rd-detail-empty">
      <span className="rd-detail-empty-icon" aria-hidden="true">
        <Braces />
      </span>
      <div className="rd-detail-empty-title">{hasEntries ? 'Select a request' : 'Nothing here yet'}</div>
      <div className="rd-detail-empty-sub">
        {hasEntries
          ? 'Pick a request on the left to inspect the full payload.'
          : 'Send a message to Pedro and every payload will show up here.'}
      </div>
    </div>
  )
}

interface DetailsBodyProps {
  conversationID?: number | null
  onEntriesChange: (n: number) => void
  /** Bumped after the backend cleared every entry. */
  clearedToken: number
}

function DetailsBody({ conversationID, onEntriesChange, clearedToken }: DetailsBodyProps) {
  const detailPaneRef = useRef<HTMLElement>(null)
  const [entries, setEntries] = useState<LLMDetailsEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setEntries((await detailsService.getAll()) ?? [])
    } catch (err) {
      toast.error('Failed to load request details', { description: String(err) })
      setEntries([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    onEntriesChange(entries.length)
  }, [entries.length, onEntriesChange])

  useEffect(() => {
    if (clearedToken === 0) return
    setEntries([])
    setSelectedId(null)
  }, [clearedToken])

  const rows = useMemo<RequestRow[]>(() => entries.map(buildRow), [entries])
  const filtered = useMemo<RequestRow[]>(() => {
    const q = filter.trim().toLowerCase()
    return q ? rows.filter(r => r.searchText.includes(q)) : rows
  }, [rows, filter])

  useEffect(() => {
    if (filtered.length === 0) {
      setSelectedId(null)
      return
    }
    setSelectedId(prev => {
      if (prev != null && filtered.some(r => r.entry.ID === prev)) return prev
      if (conversationID != null) {
        const match = filtered.find(r => r.entry.ConversationID === conversationID)
        if (match) return match.entry.ID
      }
      return filtered[0].entry.ID
    })
  }, [filtered, conversationID])

  useEffect(() => {
    detailPaneRef.current?.scrollTo({ top: 0 })
  }, [selectedId])

  const selected = useMemo(() => filtered.find(r => r.entry.ID === selectedId) ?? null, [filtered, selectedId])

  return (
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
          <div className="rd-detail-loading">
            <div className="skeleton rd-skeleton-title" />
            <div className="skeleton rd-skeleton-strip" />
            <div className="skeleton rd-skeleton-block" />
          </div>
        ) : selected ? (
          <RequestDetailView row={selected} />
        ) : (
          <EmptyDetail hasEntries={entries.length > 0} />
        )}
      </section>
    </div>
  )
}

interface DetailsProps {
  open: boolean
  onClose: () => void
  /** Preselects this conversation's entry when it opens. */
  conversationID?: number | null
}

export default function Details({ open, onClose, conversationID }: DetailsProps) {
  const [count, setCount] = useState(0)
  const [clearedToken, setClearedToken] = useState(0)

  async function clearAll() {
    const ok = await confirmAction({
      title: `Clear all ${count} captured ${count === 1 ? 'request' : 'requests'}?`,
      description: 'Stored payloads are deleted. New requests are captured again as you chat.',
      confirmLabel: 'Clear all',
    })
    if (!ok) return
    try {
      await detailsService.clear()
      setClearedToken(n => n + 1)
      toast.success('Request details cleared')
    } catch (err) {
      toast.error('Failed to clear details', { description: String(err) })
    }
  }

  return (
    <AppDialog
      open={open}
      onOpenChange={next => {
        if (!next) onClose()
      }}
      size="xl"
      className="rd-dialog"
      initialFocus=".rd-search input"
      icon={<Activity aria-hidden="true" />}
      title="Request details"
      description="The last payload Pedro sent to the LLM for each conversation."
      headerActions={
        <Button variant="danger-ghost" size="sm" onClick={() => void clearAll()} disabled={count === 0}>
          <Trash2 aria-hidden="true" />
          Clear all
        </Button>
      }
    >
      <DetailsBody conversationID={conversationID} onEntriesChange={setCount} clearedToken={clearedToken} />
    </AppDialog>
  )
}
