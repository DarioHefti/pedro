import { useState, useEffect } from 'react'
import { useTheme } from './ThemeContext'
import { main } from '../wailsjs/go/models'
import { SearchMessages } from '../wailsjs/go/main/App'

interface SidebarProps {
  conversations: main.Conversation[]
  currentConvID: number | null
  onSelect: (id: number) => void
  onNew: () => void
  onDelete: (id: number) => void
  onOpenSettings: () => void
}

export default function Sidebar({
  conversations,
  currentConvID,
  onSelect,
  onNew,
  onDelete,
  onOpenSettings,
}: SidebarProps) {
  const { theme, toggleTheme } = useTheme()
  const [searchQuery, setSearchQuery] = useState('')
  const [searchConvIDs, setSearchConvIDs] = useState<Set<number>>(new Set())

  useEffect(() => {
    async function search() {
      if (searchQuery.trim()) {
        const results = await SearchMessages(searchQuery.trim())
        setSearchConvIDs(new Set(Object.keys(results).map(Number)))
      } else {
        setSearchConvIDs(new Set())
      }
    }
    search()
  }, [searchQuery])

  const filteredConversations = searchQuery.trim()
    ? conversations.filter(c => 
        c.Title?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        searchConvIDs.has(c.ID)
      )
    : conversations

  return (
    <div className="sidebar">
      <button className="new-chat-btn" onClick={onNew}>+ New Chat</button>
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
        {filteredConversations.length === 0
          ? <div className="empty-list">
              {searchQuery ? 'No matching chats' : 'No conversations'}
            </div>
          : filteredConversations.map(conv => (
            <div
              key={conv.ID}
              className={`conversation-item${currentConvID === conv.ID ? ' active' : ''}`}
              onClick={() => onSelect(conv.ID)}
            >
              <div className="conversation-title">{conv.Title || 'New Chat'}</div>
              <button
                className="delete-btn"
                onClick={e => { e.stopPropagation(); onDelete(conv.ID) }}
              >×</button>
            </div>
          ))
        }
      </div>
      <div className="sidebar-footer">
        <button className="settings-btn" onClick={onOpenSettings}>⚙ Settings</button>
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
