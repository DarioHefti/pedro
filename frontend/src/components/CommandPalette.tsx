import { useCallback, useMemo, useState } from 'react'
import {
  Activity,
  MessageSquareText,
  MessagesSquare,
  Moon,
  PanelLeft,
  Search,
  Settings,
  SquarePen,
  Sun,
  type LucideIcon,
} from 'lucide-react'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandGroupLabel,
  CommandInput,
  CommandItem,
  CommandItemIcon,
  CommandItemText,
  CommandList,
  CommandShortcut,
} from '@ui-sdk/react/command'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import type { Conversation, Message } from '../services/wailsService'
import { AppDialog } from '../ui/dialog'
import { SHORTCUTS } from '../shortcuts'

export type PaletteAction = 'new-chat' | 'settings' | 'details' | 'toggle-theme' | 'toggle-sidebar'

interface CommandPaletteProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  conversations: Conversation[]
  theme: 'light' | 'dark'
  onAction: (action: PaletteAction) => void
  onSelectConversation: (id: number) => void
  onSearchMessages: (query: string) => Promise<Record<number, Message[]>>
}

interface ActionEntry {
  value: PaletteAction
  label: string
  icon: LucideIcon
  shortcut?: string
  keywords?: string[]
}

interface ContentHit {
  conv: Conversation
  snippet: string
}

const SNIPPET_RADIUS = 48
const MAX_RECENT = 40
const MAX_CONTENT_HITS = 12
const NO_HITS: ContentHit[] = []

function snippetFor(messages: Message[], query: string): string {
  const q = query.toLowerCase()
  for (const m of messages) {
    const text = (m.Content || '').replace(/\s+/g, ' ')
    const at = text.toLowerCase().indexOf(q)
    if (at < 0) continue
    const start = Math.max(0, at - SNIPPET_RADIUS)
    const end = Math.min(text.length, at + q.length + SNIPPET_RADIUS)
    return `${start > 0 ? '…' : ''}${text.slice(start, end)}${end < text.length ? '…' : ''}`
  }
  return ''
}

function PaletteBody({
  conversations,
  theme,
  onAction,
  onSelectConversation,
  onSearchMessages,
  close,
}: Omit<CommandPaletteProps, 'open' | 'onOpenChange'> & { close: () => void }) {
  const [query, setQuery] = useState('')
  const searchHits = useCallback(
    async (q: string): Promise<ContentHit[]> => {
      const results = await onSearchMessages(q)
      const byId = new Map(conversations.map(c => [c.ID, c]))
      return Object.entries(results)
        .map(([id, msgs]) => ({ conv: byId.get(Number(id)), snippet: snippetFor(msgs, q) }))
        .filter((h): h is ContentHit => !!h.conv && !!h.snippet)
        .slice(0, MAX_CONTENT_HITS)
    },
    [conversations, onSearchMessages],
  )
  const contentHits = useDebouncedSearch(query, searchHits, NO_HITS, { delayMs: 180, minLength: 2 })

  const actions = useMemo<ActionEntry[]>(
    () => [
      { value: 'new-chat', label: 'New chat', icon: SquarePen, shortcut: SHORTCUTS.newChat, keywords: ['create', 'start'] },
      { value: 'settings', label: 'Open settings', icon: Settings, shortcut: SHORTCUTS.settings, keywords: ['preferences', 'provider', 'model'] },
      { value: 'details', label: 'Request details', icon: Activity, shortcut: SHORTCUTS.requestDetails, keywords: ['payload', 'debug', 'tokens', 'llm'] },
      {
        value: 'toggle-theme',
        label: theme === 'light' ? 'Switch to dark theme' : 'Switch to light theme',
        icon: theme === 'light' ? Moon : Sun,
        keywords: ['appearance', 'mode', 'theme'],
      },
      { value: 'toggle-sidebar', label: 'Toggle sidebar', icon: PanelLeft, shortcut: SHORTCUTS.toggleSidebar, keywords: ['hide', 'show'] },
    ],
    [theme],
  )

  return (
    <Command
      className="palette"
      query={query}
      onQueryChange={next => setQuery(next)}
      autoHighlight
      loop
      onAction={value => {
        close()
        if (value.startsWith('chat:') || value.startsWith('hit:')) {
          onSelectConversation(Number(value.slice(value.indexOf(':') + 1)))
        } else {
          onAction(value as PaletteAction)
        }
      }}
    >
      <div className="palette-search">
        <Search aria-hidden="true" />
        <CommandInput aria-label="Search chats and actions" placeholder="Search chats, messages and actions…" />
        <kbd className="kbd">esc</kbd>
      </div>
      <CommandList className="palette-list">
        <CommandGroup>
          <CommandGroupLabel>Actions</CommandGroupLabel>
          {actions.map(a => {
            const Icon = a.icon
            return (
              <CommandItem key={a.value} value={a.value} label={a.label} keywords={a.keywords}>
                <CommandItemIcon>
                  <Icon />
                </CommandItemIcon>
                <CommandItemText />
                {a.shortcut ? <CommandShortcut shortcut={a.shortcut} /> : null}
              </CommandItem>
            )
          })}
        </CommandGroup>

        {conversations.length > 0 ? (
          <CommandGroup>
            <CommandGroupLabel>Chats</CommandGroupLabel>
            {conversations.slice(0, MAX_RECENT).map(c => (
              <CommandItem key={`chat:${c.ID}`} value={`chat:${c.ID}`} label={c.Title || 'New chat'}>
                <CommandItemIcon>
                  <MessagesSquare />
                </CommandItemIcon>
                <CommandItemText />
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}

        {contentHits.length > 0 ? (
          <CommandGroup>
            <CommandGroupLabel>Found in messages</CommandGroupLabel>
            {contentHits.map(h => (
              <CommandItem
                key={`hit:${h.conv.ID}`}
                value={`hit:${h.conv.ID}`}
                label={h.conv.Title || 'New chat'}
                keywords={[query]}
                data-has-snippet=""
              >
                <CommandItemIcon>
                  <MessageSquareText />
                </CommandItemIcon>
                <span className="palette-hit">
                  <span className="palette-hit-title">{h.conv.Title || 'New chat'}</span>
                  <span className="palette-hit-snippet">{h.snippet}</span>
                </span>
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
      </CommandList>
      <CommandEmpty className="palette-empty">
        <Search aria-hidden="true" />
        No chats or actions match “{query.trim()}”
      </CommandEmpty>
      <footer className="palette-footer" aria-hidden="true">
        <span>
          <kbd className="kbd">↑</kbd>
          <kbd className="kbd">↓</kbd> navigate
        </span>
        <span>
          <kbd className="kbd">↵</kbd> open
        </span>
        <span>
          <kbd className="kbd">esc</kbd> close
        </span>
      </footer>
    </Command>
  )
}

export function CommandPalette({ open, onOpenChange, ...rest }: CommandPaletteProps) {
  return (
    <AppDialog open={open} onOpenChange={onOpenChange} title="Command palette" size="palette" bareHeader>
      <PaletteBody {...rest} close={() => onOpenChange(false)} />
    </AppDialog>
  )
}
