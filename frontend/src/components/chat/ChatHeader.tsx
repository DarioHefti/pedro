import { Activity, Coins, Command, PanelLeftOpen, SquarePen } from 'lucide-react'
import { Button } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { SHORTCUTS } from '../../shortcuts'

interface ChatHeaderProps {
  title: string
  sidebarCollapsed: boolean
  onShowSidebar: () => void
  onNewChat: () => void
  onOpenPalette: () => void
  /** Only shown for a saved conversation. */
  stats?: { requests: number; tokens: number }
  onViewDetails?: () => void
}

const compact = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 })

export function ChatHeader({
  title,
  sidebarCollapsed,
  onShowSidebar,
  onNewChat,
  onOpenPalette,
  stats,
  onViewDetails,
}: ChatHeaderProps) {
  return (
    <header className="chat-header">
      {sidebarCollapsed ? (
        <div className="chat-header-nav">
          <Hint label="Show sidebar" shortcut={SHORTCUTS.toggleSidebar} side="bottom">
            <Button variant="ghost" size="icon-sm" onClick={onShowSidebar}>
              <PanelLeftOpen aria-hidden="true" />
            </Button>
          </Hint>
          <Hint label="New chat" shortcut={SHORTCUTS.newChat} side="bottom">
            <Button variant="ghost" size="icon-sm" onClick={onNewChat}>
              <SquarePen aria-hidden="true" />
            </Button>
          </Hint>
        </div>
      ) : null}

      <h2 className="chat-title" title={title}>
        {title}
      </h2>

      <div className="chat-header-actions">
        {stats ? (
          <div className="chat-stats">
            <Hint label={`${stats.requests.toLocaleString()} LLM requests in this chat — view details`} side="bottom">
              <button type="button" className="stat-chip" onClick={onViewDetails}>
                <Activity aria-hidden="true" />
                <span className="stat-chip-value">{stats.requests.toLocaleString()}</span>
                <span className="stat-chip-label">requests</span>
              </button>
            </Hint>
            <Hint label={`${stats.tokens.toLocaleString()} tokens used in this chat`} side="bottom">
              <button type="button" className="stat-chip" onClick={onViewDetails}>
                <Coins aria-hidden="true" />
                <span className="stat-chip-value">{compact.format(stats.tokens)}</span>
                <span className="stat-chip-label">tokens</span>
              </button>
            </Hint>
          </div>
        ) : null}
        <Hint label="Command palette" shortcut={SHORTCUTS.commandPalette} side="bottom">
          <Button variant="ghost" size="icon-sm" onClick={onOpenPalette}>
            <Command aria-hidden="true" />
          </Button>
        </Hint>
      </div>
    </header>
  )
}
