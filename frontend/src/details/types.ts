import type { LLMDetailsEntry } from '../services/wailsService'

/* OpenAI API message structures (directly from the captured request JSON). */

export interface OAIPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: { url: string; detail?: string }
}

export interface OAIToolCall {
  id: string
  type: 'function'
  function: { name: string; arguments: string }
}

export type OAIRole = 'system' | 'user' | 'assistant' | 'tool'

export interface OAIMessage {
  role: OAIRole
  content: string | OAIPart[] | null
  tool_calls?: OAIToolCall[]
  tool_call_id?: string
}

export interface JsonSchemaProperty {
  type?: string | string[]
  description?: string
  enum?: unknown[]
  items?: { type?: string }
}

export interface OAIToolDef {
  type: 'function'
  function: {
    name: string
    description?: string
    parameters?: {
      type: string
      properties?: Record<string, JsonSchemaProperty>
      required?: string[]
    }
  }
}

/* Parsed payload for the UI. */

export interface ParsedPayload {
  messages: OAIMessage[]
  tools: OAIToolDef[]
  model: string
  params: Record<string, unknown>
  response: string
  /** Total characters across all request messages. */
  totalChars: number
  roleCounts: Record<OAIRole, number>
}

export interface RequestRow {
  entry: LLMDetailsEntry
  payload: ParsedPayload
  preview: string
  /** Lower-cased haystack used by the list search. */
  searchText: string
  createdAt: Date | null
}

export interface RequestGroup {
  key: string
  label: string
  rows: RequestRow[]
}
