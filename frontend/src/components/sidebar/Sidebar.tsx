import { useCallback, useMemo, useState } from 'react'
import { Ellipsis, MessagesSquare, Moon, PanelLeftClose, Search, Settings, SquarePen, Sun, Trash2, X } from 'lucide-react'
import { InputClear, InputGroup, Input } from '@ui-sdk/react/input'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@ui-sdk/react/menu'
import { SegmentedControl, SegmentedControlIndicator, SegmentedControlItem } from '@ui-sdk/react/segmented-control'
import { useDebouncedSearch } from '../../hooks/useDebouncedSearch'
import type { Conversation, Message } from '../../services/wailsService'
import { useTheme, type Theme } from '../../ThemeContext'
import { Button, buttonStyle } from '../../ui/button'
import { confirmAction } from '../../ui/confirm'
import { Hint } from '../../ui/hint'
import { Kbd } from '../../ui/kbd'
import { toast } from '../../ui/toaster'
import pedroAvatar from '../../assets/images/pedro.svg'
import { ConversationList } from './ConversationList'
import { SHORTCUTS } from '../../shortcuts'

interface SidebarProps {
  conversations: Conversation[]
  currentConvID: number | null
  onSelect: (id: number) => void
  onNew: () => void
  onDelete: (id: number) => void
  onDeleteAllChats: () => Promise<void>
  /** Bulk delete is unavailable while any reply streams. */
  streamingBusy: boolean
  onOpenSettings: () => void
  onCollapse: () => void
  /** Returns a map of convID → matching messages. */
  onSearch: (query: string) => Promise<Record<number, Message[]>>
}

const SEARCH_DEBOUNCE_MS = 160
const NO_MATCHES: ReadonlySet<number> = new Set()

export default function Sidebar({
  conversations,
  currentConvID,
  onSelect,
  onNew,
  onDelete,
  onDeleteAllChats,
  streamingBusy,
  onOpenSettings,
  onCollapse,
  onSearch,
}: SidebarProps) {
  const { theme, setTheme } = useTheme()
  const [query, setQuery] = useState('')
  const searchContent = useCallback(
    (q: string) => onSearch(q).then(results => new Set(Object.keys(results).map(Number))),
    [onSearch],
  )
  /** Chats whose messages match the query (titles are matched locally). */
  const contentMatches = useDebouncedSearch(query, searchContent, NO_MATCHES, { delayMs: SEARCH_DEBOUNCE_MS })

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return conversations
    return conversations.filter(c => c.Title?.toLowerCase().includes(q) || contentMatches.has(c.ID))
  }, [conversations, query, contentMatches])

  async function deleteAll() {
    const count = conversations.length
    const ok = await confirmAction({
      title: `Delete all ${count} ${count === 1 ? 'chat' : 'chats'}?`,
      description: "Every conversation is removed from this device. This can't be undone.",
      confirmLabel: 'Delete all',
    })
    if (!ok) return
    try {
      await onDeleteAllChats()
      toast.success('All chats deleted')
    } catch (err) {
      toast.error('Failed to delete chats', { description: String(err) })
    }
  }

  return (
    <aside className="sidebar" aria-label="Chats">
      <header className="sidebar-header">
        <div className="brand">
          <span className="avatar brand-avatar" data-ring="">
            <img src={pedroAvatar} alt="" />
          </span>
          <span className="brand-name">Pedro</span>
        </div>
        <Hint label="Hide sidebar" shortcut={SHORTCUTS.toggleSidebar} side="bottom">
          <Button variant="ghost" size="icon-sm" onClick={onCollapse}>
            <PanelLeftClose aria-hidden="true" />
          </Button>
        </Hint>
      </header>

      <div className="sidebar-actions">
        <Button variant="primary" block align="start" onClick={onNew} className="new-chat-btn">
          <SquarePen aria-hidden="true" />
          <span>New chat</span>
          <Kbd shortcut={SHORTCUTS.newChat} />
        </Button>

        {conversations.length > 0 ? (
          <InputGroup className="sidebar-search">
            <Search className="sidebar-search-icon" aria-hidden="true" />
            <Input
              value={query}
              onValueChange={next => setQuery(next)}
              placeholder="Search chats"
              aria-label="Search chats"
              spellCheck={false}
            />
            <InputClear aria-label="Clear search">
              <X aria-hidden="true" />
            </InputClear>
          </InputGroup>
        ) : null}
      </div>

      <div className="sidebar-section-head">
        <span className="sidebar-section-title">
          {query.trim() ? 'Results' : 'Chats'}
          <span className="sidebar-section-count">{query.trim() ? filtered.length : conversations.length}</span>
        </span>
        {conversations.length > 0 ? (
          <Menu onAction={value => (value === 'delete-all' ? void deleteAll() : undefined)}>
            <Hint label="Chat list options">
              <MenuTrigger {...buttonStyle({ variant: 'ghost', size: 'icon-xs' })}>
                <Ellipsis aria-hidden="true" />
              </MenuTrigger>
            </Hint>
            <MenuContent align="end" sideOffset={6}>
              <MenuItem value="delete-all" destructive disabled={streamingBusy}>
                <Trash2 aria-hidden="true" />
                Delete all chats…
              </MenuItem>
            </MenuContent>
          </Menu>
        ) : null}
      </div>

      {conversations.length === 0 ? (
        <div className="empty-hint sidebar-empty">
          <MessagesSquare aria-hidden="true" />
          <strong>No chats yet</strong>
          <span>Your conversations will show up here.</span>
        </div>
      ) : filtered.length === 0 ? (
        <div className="empty-hint sidebar-empty">
          <Search aria-hidden="true" />
          <strong>No matches</strong>
          <span>Nothing found for “{query.trim()}”.</span>
        </div>
      ) : (
        <ConversationList
          conversations={filtered}
          currentConvID={currentConvID}
          onSelect={onSelect}
          onDelete={onDelete}
        />
      )}

      <footer className="sidebar-footer">
        <Hint label="Settings" shortcut={SHORTCUTS.settings}>
          <Button variant="ghost" size="sm" onClick={onOpenSettings} className="sidebar-settings-btn">
            <Settings aria-hidden="true" />
            <span>Settings</span>
          </Button>
        </Hint>
        <SegmentedControl<Theme>
          aria-label="Theme"
          value={theme}
          onValueChange={next => {
            if (next) setTheme(next)
          }}
          data-size="sm"
        >
          <SegmentedControlIndicator />
          <SegmentedControlItem value="light" aria-label="Light theme">
            <Sun aria-hidden="true" />
          </SegmentedControlItem>
          <SegmentedControlItem value="dark" aria-label="Dark theme">
            <Moon aria-hidden="true" />
          </SegmentedControlItem>
        </SegmentedControl>
      </footer>
    </aside>
  )
}
