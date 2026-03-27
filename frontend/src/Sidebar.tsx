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
  onOpenSettings,
  onSearch,
}: SidebarProps) {
  const { theme, toggleTheme } = useTheme()
  const [searchQuery, setSearchQuery] = useState('')
  const [searchConvIDs, setSearchConvIDs] = useState<Set<number>>(new Set())

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

  const filteredConversations = searchQuery.trim()
    ? conversations.filter(
        c =>
          c.Title?.toLowerCase().includes(searchQuery.toLowerCase()) ||
          searchConvIDs.has(c.ID),
      )
    : conversations

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
