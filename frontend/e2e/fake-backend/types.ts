/**
 * Contracts of the fake Go backend: what a test seeds it with, how it scripts LLM replies,
 * and the control surface it exposes on `window.__pedroFake`.
 */

export type Role = 'user' | 'assistant' | 'tool'

/** A tool call the scripted model makes before it answers. */
export interface ScriptedToolCall {
  name: string
  args: Record<string, unknown>
  /** What the tool "returned"; stored as the tool message. */
  result?: string
}

/** One scripted LLM turn. The fake streams it exactly like the Go backend streams a provider reply. */
export interface ReplyScript {
  /** Tool calls made in a first round-trip, before the answer. */
  toolCalls?: ScriptedToolCall[]
  /** The final answer, streamed in chunks. */
  text?: string
  /** Fails the turn: the send resolves to `Error: <error>` and nothing but the user message is kept. */
  error?: string
  /** Characters per stream chunk. Default 12. */
  chunkSize?: number
  /** Delay between stream events in ms. Default 12. */
  chunkDelayMs?: number
  /** Delay before the first event in ms (model "thinking"). Default 40. */
  firstTokenDelayMs?: number
  /** Token usage reported per request. */
  usage?: { prompt: number; completion: number }
}

export interface SeedMessage {
  role: Role
  content: string
  /** Assistant tool calls, persisted as the backend's merged tool-call message. */
  toolCalls?: { name: string; args: Record<string, unknown>; id?: string }[]
  toolCallId?: string
  /** Composer attachments as the frontend persists them. */
  attachments?: { type: 'text' | 'image' | 'file-ref' | 'folder-ref'; content: string; name: string }[]
}

export interface SeedConversation {
  title: string
  /** How long ago the chat was last updated; drives the sidebar's recency groups. */
  daysAgo?: number
  messages: SeedMessage[]
  requestCount?: number
  requestTokens?: number
}

export interface SeedMemory {
  key: string
  value: string
  category?: string
  importance?: number
}

export interface SeedUpdate {
  latestVersion: string
  assetName?: string
  assetURL?: string
}

export interface FakeSeed {
  /** Merged over the defaults (an OpenAI-compatible provider that is ready to chat). */
  settings?: Record<string, string>
  /** Start without a configured provider, as on first launch. */
  unconfigured?: boolean
  conversations?: SeedConversation[]
  memories?: SeedMemory[]
  /** Replies used, in order, before the keyword-based default responder. */
  replies?: ReplyScript[]
  /** Paths the native file / folder pickers return, in order. Empty means "cancelled". */
  fileSelections?: string[]
  folderSelections?: string[]
  /** Result of TestConnection; a string fails it with that message. */
  connectionTest?: 'ok' | { error: string }
  stats?: { globalRequests?: number; lifetimeTokens?: number }
  version?: string
  update?: SeedUpdate
  /** Simulated IPC latency for every binding call, in ms. Default 0. */
  latencyMs?: number
}

export interface RecordedCall {
  method: string
  args: unknown[]
}

export interface FakeMessage {
  ID: number
  ConversationID: number
  Role: string
  Content: string
  Attachments: string
  ToolCalls: string
  ToolCallID: string
  CreatedAt: string
}

export interface FakeConversation {
  ID: number
  Title: string
  CreatedAt: string
  UpdatedAt: string
  requestCount: number
  requestTokens: number
}

export interface FakeSnapshot {
  conversations: FakeConversation[]
  messages: FakeMessage[]
  settings: Record<string, string>
  memories: { ID: number; Key: string; Value: string; Category: string; Importance: number }[]
  llmDetails: number
}

/** Exposed on `window.__pedroFake` so tests can drive and inspect the backend. */
export interface FakeControl {
  queueReply(reply: ReplyScript): void
  queueFileSelection(path: string): void
  queueFolderSelection(path: string): void
  /** Simulates a native file drop (Wails OnFileDrop). */
  dropFiles(paths: string[]): void
  /** Emits a runtime event as the Go side would. */
  emit(event: string, ...data: unknown[]): void
  calls(method?: string): RecordedCall[]
  snapshot(): FakeSnapshot
}
