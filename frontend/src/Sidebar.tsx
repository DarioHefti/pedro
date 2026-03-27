import { useState, useEffect } from 'react'
import { useTheme } from './ThemeContext'
import type { Conversation, Message } from './services/wailsService'
import { uiConversationService } from './services/wailsService'

interface SidebarProps {
  conversations: Conversation[]
  currentConvID: number | null
  onSelect: (id: number) => void
  onNew: () => void
  onDelete: (id: number) => void
  /** Deletes every stored chat; resolves when the list has been refreshed. */
  onDeleteAllChats: () => Promise<void>
  /** When true, bulk delete controls are disabled and any confirm step is cancelled. */
  streamingBusy: boolean
  onOpenSettings: () => void
  /** Called with the search query; returns a map of convID → matching messages. */
  onSearch: (query: string) => Promise<Record<number, Message[]>>
}

export default function Sidebar({
  conversations,
  currentConvID,
  onSelect,
  onNew,
  onDelete,
  onDeleteAllChats,
  streamingBusy,
  onOpenSettings,
  onSearch,
}: SidebarProps) {
  const { theme, toggleTheme } = useTheme()
  const [searchQuery, setSearchQuery] = useState('')
  const [searchConvIDs, setSearchConvIDs] = useState<Set<number>>(new Set())
  const [deleteAllStep, setDeleteAllStep] = useState<'idle' | 'confirm'>('idle')
  const [deleteAllPending, setDeleteAllPending] = useState(false)

  useEffect(() => {
    async function search() {
      if (searchQuery.trim()) {
        const results = await onSearch(searchQuery.trim())
        setSearchConvIDs(new Set(Object.keys(results).map(Number)))
      } else {
        setSearchConvIDs(new Set())
      }
    }
    search()
  }, [searchQuery, onSearch])

  useEffect(() => {
    if (streamingBusy) setDeleteAllStep('idle')
  }, [streamingBusy])

  useEffect(() => {
    if (conversations.length === 0) setDeleteAllStep('idle')
  }, [conversations.length])

  const filteredConversations = searchQuery.trim()
    ? conversations.filter(
        c =>
          c.Title?.toLowerCase().includes(searchQuery.toLowerCase()) ||
          searchConvIDs.has(c.ID),
      )
    : conversations

  const chatCount = conversations.length
  const chatsLabel = chatCount === 1 ? '1 chat' : `${chatCount} chats`
  const deleteAllDisabled = streamingBusy || deleteAllPending

  async function handleConfirmDeleteAll() {
    setDeleteAllPending(true)
    try {
      await onDeleteAllChats()
      setDeleteAllStep('idle')
    } finally {
      setDeleteAllPending(false)
    }
  }

  return (
    <div className="sidebar">
      <button className="new-chat-btn" onClick={onNew}>
        <span className="new-chat-btn-label">New chat</span>
        <span className="new-chat-btn-icon" aria-hidden="true">+</span>
      </button>
      {conversations.length > 0 && (
        <div className="sidebar-search">
          <input
            type="text"
            placeholder="Search chats..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="sidebar-search-input"
          />
        </div>
      )}
      <div className="conversation-list">
        {filteredConversations.length === 0 ? (
          <div className="empty-list">
            {searchQuery ? 'No matching chats' : 'No conversations'}
          </div>
        ) : (
          filteredConversations.map(conv => (
            <div
              key={conv.ID}
              className={`conversation-item${currentConvID === conv.ID ? ' active' : ''}`}
              onClick={() => onSelect(conv.ID)}
            >
              <div className="conversation-title">{conv.Title || 'New Chat'}</div>
              {uiConversationService.canDeleteConversation(conv.ID) ? (
                <button
                  className="delete-btn"
                  onClick={e => {
                    e.stopPropagation()
                    onDelete(conv.ID)
                  }}
                >
                  ×
                </button>
              ) : null}
            </div>
          ))
        )}
      </div>

      {chatCount > 0 && (
        <>
          {deleteAllStep === 'idle' ? (
            <div className="sidebar-bulk-delete-strip">
              <span className="sidebar-bulk-delete-count">{chatsLabel}</span>
              <button
                type="button"
                className="sidebar-bulk-delete-trigger"
                disabled={deleteAllDisabled}
                onClick={() => setDeleteAllStep('confirm')}
              >
                Delete all
              </button>
            </div>
          ) : (
            <div
              className="sidebar-bulk-delete-confirm"
              role="group"
              aria-label="Confirm delete all chats"
            >
              <p className="sidebar-bulk-delete-confirm-text">
                Remove every chat from the list? This can&apos;t be undone.
              </p>
              <div className="sidebar-bulk-delete-actions">
                <button
                  type="button"
                  className="sidebar-bulk-delete-btn-cancel"
                  disabled={deleteAllPending}
                  onClick={() => setDeleteAllStep('idle')}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="sidebar-bulk-delete-btn-confirm"
                  disabled={deleteAllDisabled}
                  onClick={() => void handleConfirmDeleteAll()}
                >
                  Delete all chats
                </button>
              </div>
            </div>
          )}
        </>
      )}

      <div className="sidebar-footer">
        <button className="settings-btn" onClick={onOpenSettings}>
          ⚙ Settings
        </button>
        <button
          className="theme-toggle-btn"
          onClick={toggleTheme}
          title={theme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'}
        >
          {theme === 'light' ? '🌙' : '☀️'}
        </button>
      </div>
    </div>
  )
}
