import { useMemo } from 'react'
import { Trash2 } from 'lucide-react'
import type { Conversation } from '../../services/wailsService'
import { Button } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { groupConversations } from './conversationGroups'

interface ConversationListProps {
  conversations: Conversation[]
  currentConvID: number | null
  onSelect: (id: number) => void
  onDelete: (id: number) => void
}

/** Recency-grouped chat list. Hovering a row shows a one-click delete button at its end. */
export function ConversationList({ conversations, currentConvID, onSelect, onDelete }: ConversationListProps) {
  const groups = useMemo(() => groupConversations(conversations), [conversations])

  return (
    <nav className="conv-list" aria-label="Chat history">
      {groups.map(group => (
        <section key={group.key} className="conv-group" aria-label={group.label}>
          <h3 className="conv-group-label">{group.label}</h3>
          <ul className="conv-group-items">
            {group.conversations.map(conv => {
              const active = conv.ID === currentConvID
              const title = conv.Title || 'New chat'
              return (
                <li key={conv.ID} className="conv-row" data-active={active ? '' : undefined}>
                  <button
                    type="button"
                    className="conv-row-main"
                    aria-current={active ? 'page' : undefined}
                    onClick={() => onSelect(conv.ID)}
                    title={title}
                  >
                    <span className="conv-row-title">{title}</span>
                  </button>
                  <Hint label="Delete chat">
                    <Button
                      variant="danger-quiet"
                      size="icon-xs"
                      className="conv-row-delete"
                      aria-label={`Delete ${title}`}
                      onClick={() => onDelete(conv.ID)}
                    >
                      <Trash2 aria-hidden="true" />
                    </Button>
                  </Hint>
                </li>
              )
            })}
          </ul>
        </section>
      ))}
    </nav>
  )
}
