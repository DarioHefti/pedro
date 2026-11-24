import {
  AudioLines,
  Brain,
  FileSearch,
  FileText,
  FolderTree,
  Globe,
  Image,
  ScanSearch,
  Search,
  Wrench,
  type LucideIcon,
} from 'lucide-react'
import type { ToolCall } from '../../hooks/messageModel'

interface ToolMeta {
  label: string
  icon: LucideIcon
}

const TOOL_META: Record<string, ToolMeta> = {
  web_search: { label: 'Web search', icon: Search },
  websearch: { label: 'Web search', icon: Search },
  site_search: { label: 'Site search', icon: ScanSearch },
  web_find: { label: 'Find on page', icon: ScanSearch },
  webclaw: { label: 'Read web page', icon: Globe },
  webfetch: { label: 'Read web page', icon: Globe },
  browser: { label: 'Browser', icon: Globe },
  read_file: { label: 'Read file', icon: FileText },
  parse_document: { label: 'Parse document', icon: FileText },
  grep: { label: 'Search files', icon: FileSearch },
  glob: { label: 'Find files', icon: FileSearch },
  show_file_tree: { label: 'Browse folder', icon: FolderTree },
  memory_save: { label: 'Remember', icon: Brain },
  memory_search: { label: 'Recall memory', icon: Brain },
  memory_forget: { label: 'Forget memory', icon: Brain },
  generate_image: { label: 'Generate image', icon: Image },
  edit_image: { label: 'Edit image', icon: Image },
  transcribe_audio: { label: 'Transcribe audio', icon: AudioLines },
  tool_search: { label: 'Discover tools', icon: Wrench },
}

export function toolMeta(name: string): ToolMeta {
  return TOOL_META[name] ?? { label: name.replace(/_/g, ' '), icon: Wrench }
}

const ARG_KEYS = ['query', 'url', 'prompt', 'path', 'pattern', 'include', 'key'] as const

/** The one argument that best describes a call, like the search query or the file path. */
export function toolCallArgDisplay(tc: ToolCall): string {
  try {
    const args = JSON.parse(tc.argsJSON) as Record<string, unknown>
    for (const key of ARG_KEYS) {
      const v = args[key]
      if (typeof v === 'string' && v.trim()) return key === 'query' ? `“${v}”` : v
    }
  } catch {
    /* arguments may still be streaming */
  }
  return ''
}

/** Distinct tool labels in first-use order, for a compact summary. */
export function summarizeTools(calls: ToolCall[]): string[] {
  const seen = new Set<string>()
  const labels: string[] = []
  for (const c of calls) {
    const label = toolMeta(c.name).label
    if (!seen.has(label)) {
      seen.add(label)
      labels.push(label)
    }
  }
  return labels
}
