import { FileText, FolderTree, Globe, Workflow, type LucideIcon } from 'lucide-react'
import pedroAvatar from '../../assets/images/pedro.svg'

export type SuggestionAttach = 'file' | 'folder'

interface Suggestion {
  id: string
  icon: LucideIcon
  title: string
  hint: string
  prompt: string
  attach?: SuggestionAttach
}

const SUGGESTIONS: Suggestion[] = [
  {
    id: 'research',
    icon: Globe,
    title: 'Research a topic',
    hint: 'Searches the web and cites sources',
    prompt: 'Research the latest developments on ',
  },
  {
    id: 'folder',
    icon: FolderTree,
    title: 'Explain a project',
    hint: 'Pick a folder to explore',
    prompt: 'Give me a clear overview of this project: what it does, how it is structured, and where to start.',
    attach: 'folder',
  },
  {
    id: 'document',
    icon: FileText,
    title: 'Summarize a document',
    hint: 'PDF, Word, text or audio',
    prompt: 'Summarize the key points of this file in a few bullet points.',
    attach: 'file',
  },
  {
    id: 'diagram',
    icon: Workflow,
    title: 'Draw a diagram',
    hint: 'Mermaid flowcharts & sequences',
    prompt: 'Create a mermaid diagram that explains ',
  },
]

interface EmptyStateProps {
  welcomeMessage: string
  onPick: (prompt: string, attach?: SuggestionAttach) => void
}

export function EmptyState({ welcomeMessage, onPick }: EmptyStateProps) {
  return (
    <div className="empty-state">
      <div className="empty-hero">
        <span className="empty-avatar" aria-hidden="true">
          <span className="empty-avatar-halo" />
          <img src={pedroAvatar} alt="" />
        </span>
        <h1 className="empty-title">{welcomeMessage}</h1>
        <p className="empty-subtitle">Ask anything, attach files or folders, or start from one of these.</p>
      </div>
      <ul className="suggestions" aria-label="Suggestions">
        {SUGGESTIONS.map(s => {
          const Icon = s.icon
          return (
            <li key={s.id}>
              <button type="button" className="suggestion" onClick={() => onPick(s.prompt, s.attach)}>
                <span className="suggestion-icon" aria-hidden="true">
                  <Icon />
                </span>
                <span className="suggestion-text">
                  <span className="suggestion-title">{s.title}</span>
                  <span className="suggestion-hint">{s.hint}</span>
                </span>
              </button>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
