import type { LLMDetailsEntry } from '../services/wailsService'
import type {
  JsonSchemaProperty,
  OAIMessage,
  OAIPart,
  OAIRole,
  OAIToolDef,
  ParsedPayload,
  RequestGroup,
  RequestRow,
} from './types'

/* Hard cap on how many characters we ever hand to the DOM for a single block. */
export const MAX_RENDER = 24000
/* Only scan this many chars when building a collapsed preview. */
export const PREVIEW_SCAN = 1600
/* Messages longer than this start collapsed. */
export const COLLAPSE_THRESHOLD = 600

const INTERESTING_PARAMS = [
  'temperature', 'max_tokens', 'max_completion_tokens', 'top_p',
  'frequency_penalty', 'presence_penalty', 'seed', 'n', 'stop',
  'response_format', 'tool_choice', 'parallel_tool_calls',
  'reasoning_effort', 'service_tier',
]

/* -------------------------------------------------------------------------- */
/*  Formatting                                                                 */
/* -------------------------------------------------------------------------- */

export function toDate(raw: unknown): Date | null {
  if (raw == null || raw === '') return null
  const d = new Date(raw as string)
  return Number.isNaN(d.getTime()) ? null : d
}

const timeFmt = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' })
const fullFmt = new Intl.DateTimeFormat(undefined, {
  weekday: 'short',
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
})
const dayFmt = new Intl.DateTimeFormat(undefined, { weekday: 'long', month: 'short', day: 'numeric' })

export function formatTime(d: Date | null): string {
  return d ? timeFmt.format(d) : '—'
}

export function formatFull(d: Date | null): string {
  return d ? fullFmt.format(d) : 'Unknown time'
}

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

function dayLabel(d: Date, now: Date): string {
  const diffDays = Math.round((startOfDay(now) - startOfDay(d)) / 86_400_000)
  if (diffDays === 0) return 'Today'
  if (diffDays === 1) return 'Yesterday'
  return dayFmt.format(d)
}

export function humanSize(len: number): string {
  if (len < 1000) return `${len} chars`
  if (len < 1_000_000) return `${(len / 1000).toFixed(len < 10_000 ? 1 : 0)}k chars`
  return `${(len / 1_000_000).toFixed(1)}M chars`
}

export function compactNumber(n: number): string {
  if (n < 1000) return String(n)
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0)}k`
  return `${(n / 1_000_000).toFixed(1)}M`
}

export function prettyJson(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

export function formatValue(v: unknown): string {
  return typeof v === 'string' ? v : JSON.stringify(v)
}

/* -------------------------------------------------------------------------- */
/*  Payload parsing                                                            */
/* -------------------------------------------------------------------------- */

/** Normalize any OpenAI content shape to a plain string. */
export function normalizeContent(c: string | OAIPart[] | null | undefined): string {
  if (c == null) return ''
  if (typeof c === 'string') return c
  return c
    .map(p => {
      if (p.type === 'text') return p.text ?? ''
      if (p.type === 'image_url') return `[image: ${p.image_url?.url ?? ''}]`
      return ''
    })
    .filter(Boolean)
    .join('\n')
}

function extractParams(request: Record<string, unknown>): Record<string, unknown> {
  const params: Record<string, unknown> = {}
  for (const key of INTERESTING_PARAMS) {
    if (request[key] !== undefined && request[key] !== null) {
      params[key] = request[key]
    }
  }
  return params
}

function emptyRoleCounts(): Record<OAIRole, number> {
  return { system: 0, user: 0, assistant: 0, tool: 0 }
}

export function parsePayload(json: string): ParsedPayload {
  const empty: ParsedPayload = {
    messages: [], tools: [], model: '', params: {}, response: '',
    totalChars: 0, roleCounts: emptyRoleCounts(),
  }
  try {
    const p = JSON.parse(json)
    const req = p.request
    if (!req || typeof req !== 'object') return empty

    const messages: OAIMessage[] = Array.isArray(req.messages) ? req.messages : []
    const roleCounts = emptyRoleCounts()
    let totalChars = 0
    for (const m of messages) {
      if (m.role in roleCounts) roleCounts[m.role] += 1
      totalChars += normalizeContent(m.content).length
    }

    return {
      messages,
      tools: Array.isArray(req.tools) ? req.tools : [],
      model: typeof req.model === 'string' ? req.model : '',
      params: extractParams(req),
      response: typeof p.response === 'string' ? p.response : '',
      totalChars,
      roleCounts,
    }
  } catch {
    return empty
  }
}

function buildPreview(payload: ParsedPayload): string {
  const lastUser = [...payload.messages].reverse().find(m => m.role === 'user')
  const content = lastUser ? normalizeContent(lastUser.content) : ''
  const source = content || payload.response || normalizeContent(payload.messages[0]?.content) || ''
  const flat = source.slice(0, 240).replace(/\s+/g, ' ').trim()
  return flat || 'No content'
}

function buildSearchText(entry: LLMDetailsEntry, payload: ParsedPayload): string {
  const parts = [entry.Model, entry.ConversationTitle, payload.response]
  for (const m of payload.messages) {
    parts.push(m.role, normalizeContent(m.content))
  }
  return parts.join('\n').toLowerCase()
}

export function buildRow(entry: LLMDetailsEntry): RequestRow {
  const payload = parsePayload(entry.Messages)
  return {
    entry,
    payload,
    preview: buildPreview(payload),
    searchText: buildSearchText(entry, payload),
    createdAt: toDate(entry.CreatedAt),
  }
}

/** Group rows (already sorted newest-first) by calendar day. */
export function groupByDay(rows: RequestRow[], now = new Date()): RequestGroup[] {
  const groups: RequestGroup[] = []
  for (const row of rows) {
    const key = row.createdAt ? String(startOfDay(row.createdAt)) : 'unknown'
    let group = groups[groups.length - 1]
    if (!group || group.key !== key) {
      group = { key, label: row.createdAt ? dayLabel(row.createdAt, now) : 'Unknown date', rows: [] }
      groups.push(group)
    }
    group.rows.push(row)
  }
  return groups
}

/* -------------------------------------------------------------------------- */
/*  Roles & tools                                                              */
/* -------------------------------------------------------------------------- */

export const ROLE_ORDER: OAIRole[] = ['system', 'user', 'assistant', 'tool']

export function roleLabel(role: string): string {
  switch (role) {
    case 'system': return 'System'
    case 'user': return 'User'
    case 'assistant': return 'Pedro'
    case 'tool': return 'Tool result'
    default: return role || 'Unknown'
  }
}

export function roleInitial(role: string): string {
  switch (role) {
    case 'system': return 'S'
    case 'user': return 'U'
    case 'assistant': return 'P'
    case 'tool': return 'T'
    default: return '?'
  }
}

export interface ToolParamSummary {
  name: string
  type: string
  required: boolean
  description: string
}

function schemaType(prop: JsonSchemaProperty): string {
  const t = Array.isArray(prop.type) ? prop.type.join(' | ') : prop.type ?? 'any'
  if (t === 'array' && prop.items?.type) return `${prop.items.type}[]`
  if (prop.enum && prop.enum.length > 0) return prop.enum.map(v => JSON.stringify(v)).join(' | ')
  return t
}

export function summarizeToolParams(tool: OAIToolDef): ToolParamSummary[] {
  const props = tool.function?.parameters?.properties ?? {}
  const required = new Set(tool.function?.parameters?.required ?? [])
  return Object.entries(props)
    .map(([name, prop]) => ({
      name,
      type: schemaType(prop ?? {}),
      required: required.has(name),
      description: prop?.description ?? '',
    }))
    .sort((a, b) => Number(b.required) - Number(a.required))
}
