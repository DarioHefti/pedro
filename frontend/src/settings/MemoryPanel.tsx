import { useCallback, useEffect, useMemo, useState } from 'react'
import { Brain, Check, Pencil, Plus, Search, Trash2, X } from 'lucide-react'
import { Input, InputClear, InputGroup } from '@ui-sdk/react/input'
import type { MemoryRecord } from '../services/wailsService'
import { Button } from '../ui/button'
import { TextField } from '../ui/fields'
import { Hint } from '../ui/hint'
import { toast } from '../ui/toaster'
import type { SettingsServices } from './settingsModel'

interface MemoryPanelProps {
  services: Pick<SettingsServices, 'getMemories' | 'saveMemory' | 'deleteMemory'>
}

interface MemoryForm {
  key: string
  value: string
  category: string
}

const EMPTY_FORM: MemoryForm = { key: '', value: '', category: '' }

function ImportanceBadge({ importance }: { importance: number }) {
  if (importance >= 5) return <span data-badge="danger">Critical</span>
  if (importance >= 4) return <span data-badge="warning">Important</span>
  return null
}

export function MemoryPanel({ services }: MemoryPanelProps) {
  const [memories, setMemories] = useState<MemoryRecord[]>([])
  const [loaded, setLoaded] = useState(false)
  const [form, setForm] = useState<MemoryForm>(EMPTY_FORM)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [edit, setEdit] = useState<MemoryForm>(EMPTY_FORM)
  const [filter, setFilter] = useState('')

  const load = useCallback(async () => {
    try {
      setMemories((await services.getMemories()) ?? [])
    } catch (e) {
      toast.error('Failed to load memories', { description: String(e) })
    } finally {
      setLoaded(true)
    }
  }, [services])

  useEffect(() => {
    void load()
  }, [load])

  const visible = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return memories
    return memories.filter(m => `${m.Key} ${m.Value} ${m.Category}`.toLowerCase().includes(q))
  }, [memories, filter])

  async function add() {
    const key = form.key.trim()
    const value = form.value.trim()
    if (!key || !value) {
      toast.error('Key and value are required')
      return
    }
    try {
      await services.saveMemory(key, value, form.category.trim())
      setForm(EMPTY_FORM)
      await load()
    } catch (e) {
      toast.error('Could not save memory', { description: String(e) })
    }
  }

  async function saveEdit() {
    const key = edit.key.trim()
    const value = edit.value.trim()
    if (editingId == null || !key || !value) return
    try {
      await services.saveMemory(key, value, edit.category.trim())
      setEditingId(null)
      await load()
    } catch (e) {
      toast.error('Could not save memory', { description: String(e) })
    }
  }

  async function remove(m: MemoryRecord) {
    try {
      await services.deleteMemory(m.ID)
      if (editingId === m.ID) setEditingId(null)
      await load()
      toast.success(`Forgot “${m.Key}”`)
    } catch (e) {
      toast.error('Could not delete memory', { description: String(e) })
    }
  }

  return (
    <div className="settings-stack">
      <div className="settings-card">
        <div className="settings-card-head">
          <Plus aria-hidden="true" />
          <span>Teach Pedro something</span>
        </div>
        <div className="memory-form">
          <TextField label="Key" value={form.key} onValueChange={key => setForm({ ...form, key })} placeholder="user_name" mono />
          <TextField label="Value" value={form.value} onValueChange={value => setForm({ ...form, value })} placeholder="Sarah" />
          <TextField label="Category" optional value={form.category} onValueChange={category => setForm({ ...form, category })} placeholder="personal" />
          <Button variant="primary" onClick={add} disabled={!form.key.trim() || !form.value.trim()} className="memory-form-submit">
            <Plus aria-hidden="true" />
            Add
          </Button>
        </div>
      </div>

      <div className="settings-list-head">
        <span>Saved memories</span>
        <span className="tab-count">{memories.length}</span>
        {memories.length > 4 ? (
          <InputGroup className="settings-list-filter">
            <Search className="input-leading-icon" aria-hidden="true" />
            <Input value={filter} onValueChange={v => setFilter(v)} placeholder="Filter" aria-label="Filter memories" />
            <InputClear aria-label="Clear filter">
              <X aria-hidden="true" />
            </InputClear>
          </InputGroup>
        ) : null}
      </div>

      {!loaded ? (
        <div className="skeleton settings-skeleton" />
      ) : memories.length === 0 ? (
        <div className="empty-hint settings-empty">
          <Brain aria-hidden="true" />
          <strong>Nothing remembered yet</strong>
          <span>Pedro saves facts about you while you chat. They'll show up here.</span>
        </div>
      ) : visible.length === 0 ? (
        <div className="empty-hint settings-empty">
          <Search aria-hidden="true" />
          <span>No memories match “{filter.trim()}”.</span>
        </div>
      ) : (
        <ul className="settings-list">
          {visible.map(m =>
            editingId === m.ID ? (
              <li key={m.ID} className="settings-list-item" data-editing="">
                <div className="memory-form memory-form-edit">
                  <div className="memory-key-static">
                    <span className="settings-muted">Key</span>
                    <code>{edit.key}</code>
                  </div>
                  <TextField label="Value" value={edit.value} onValueChange={value => setEdit({ ...edit, value })} autoFocus />
                  <TextField label="Category" optional value={edit.category} onValueChange={category => setEdit({ ...edit, category })} />
                  <div className="memory-edit-actions">
                    <Hint label="Cancel">
                      <Button variant="ghost" size="icon-sm" onClick={() => setEditingId(null)}>
                        <X aria-hidden="true" />
                      </Button>
                    </Hint>
                    <Hint label="Save">
                      <Button variant="primary" size="icon-sm" onClick={saveEdit} disabled={!edit.value.trim()}>
                        <Check aria-hidden="true" />
                      </Button>
                    </Hint>
                  </div>
                </div>
              </li>
            ) : (
              <li key={m.ID} className="settings-list-item">
                <div className="settings-list-text">
                  <span className="memory-row-head">
                    <code className="memory-key">{m.Key}</code>
                    <ImportanceBadge importance={m.Importance} />
                    {m.Category ? <span data-badge="outline">{m.Category}</span> : null}
                  </span>
                  <span title={m.Value}>{m.Value}</span>
                </div>
                <div className="settings-list-actions">
                  <Hint label="Edit">
                    <Button
                      variant="ghost"
                      size="icon-xs"
                      onClick={() => {
                        setEditingId(m.ID)
                        setEdit({ key: m.Key, value: m.Value, category: m.Category })
                      }}
                    >
                      <Pencil aria-hidden="true" />
                    </Button>
                  </Hint>
                  <Hint label="Forget">
                    <Button variant="danger-ghost" size="icon-xs" onClick={() => remove(m)}>
                      <Trash2 aria-hidden="true" />
                    </Button>
                  </Hint>
                </div>
              </li>
            ),
          )}
        </ul>
      )}
    </div>
  )
}
