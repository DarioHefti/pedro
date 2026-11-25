import type { FakeConversation, FakeMessage, FakeSeed, SeedMessage } from './types'

/** Go's zero time, which CreateConversation returns for fresh rows. */
const ZERO_TIME = '0001-01-01T00:00:00Z'
const NEW_CHAT_TITLE = 'New Chat'
const TITLE_MARKERS = ['\n\n[File:', '\n\n[Folder:', '\n\n[Path:', ' [File:', ' [Folder:', ' [Path:']

/** Mirrors database.go sanitizeConversationTitle: drops the file notes appended for the model. */
export function sanitizeConversationTitle(raw: string): string {
  let cut = raw.length
  for (const marker of TITLE_MARKERS) {
    const idx = raw.indexOf(marker)
    if (idx >= 0 && idx < cut) cut = idx
  }
  return raw.slice(0, cut).trim()
}

/** Mirrors database.go AddMessage's title rule for the first user message. */
export function titleFromFirstMessage(content: string): string {
  const title = sanitizeConversationTitle(content)
  return title.length > 50 ? `${title.slice(0, 50)}...` : title
}

export interface MemoryRow {
  ID: number
  Key: string
  Value: string
  Category: string
  Importance: number
  Source: string
  UpdatedAt: string
}

export interface LLMDetailsRow {
  ID: number
  ConversationID: number
  CreatedAt: string
  RequestCount: number
  Model: string
  Messages: string
}

/**
 * In-memory stand-in for the SQLite store (database.go). Messages are ordered by insertion,
 * and message indexes are positions in that order, exactly like the Go store.
 */
export class FakeStore {
  conversations: FakeConversation[] = []
  messages: FakeMessage[] = []
  settings: Record<string, string> = {}
  memories: MemoryRow[] = []
  llmDetails: LLMDetailsRow[] = []
  private nextId = { conversation: 1, message: 1, memory: 1, details: 1 }
  /** Monotonic clock so ordering by time never ties within a test. */
  private tick = 0

  constructor(private readonly now: () => number = () => Date.now()) {}

  private timestamp(offsetMs = 0): string {
    this.tick += 1
    return new Date(this.now() + offsetMs + this.tick).toISOString()
  }

  // ------------------------------------------------------------ conversations

  createConversation(): FakeConversation {
    const conv: FakeConversation = {
      ID: this.nextId.conversation++,
      Title: NEW_CHAT_TITLE,
      CreatedAt: this.timestamp(),
      UpdatedAt: this.timestamp(),
      requestCount: 0,
      requestTokens: 0,
    }
    this.conversations.push(conv)
    return conv
  }

  /** What CreateConversation returns over the bridge (Go leaves the times zero). */
  static createdConversationResult(conv: FakeConversation) {
    return { ID: conv.ID, Title: conv.Title, CreatedAt: ZERO_TIME, UpdatedAt: ZERO_TIME }
  }

  getConversation(id: number): FakeConversation | undefined {
    return this.conversations.find(c => c.ID === id)
  }

  /** Newest first, like `ORDER BY updated_at DESC`. */
  listConversations(): FakeConversation[] {
    return [...this.conversations].sort((a, b) => b.UpdatedAt.localeCompare(a.UpdatedAt) || b.ID - a.ID)
  }

  deleteConversation(id: number): void {
    this.llmDetails = this.llmDetails.filter(d => d.ConversationID !== id)
    this.conversations = this.conversations.filter(c => c.ID !== id)
    this.messages = this.messages.filter(m => m.ConversationID !== id)
  }

  deleteAllConversations(): void {
    this.llmDetails = []
    this.conversations = []
    this.messages = []
  }

  // ----------------------------------------------------------------- messages

  getMessages(conversationID: number): FakeMessage[] {
    return this.messages.filter(m => m.ConversationID === conversationID)
  }

  addMessage(
    conversationID: number,
    role: string,
    content: string,
    attachments = '',
    toolCalls = '',
    toolCallID = '',
  ): FakeMessage {
    const conv = this.getConversation(conversationID)
    // Foreign keys are enforced: a message needs an existing conversation.
    if (!conv) throw new Error('FOREIGN KEY constraint failed')
    const msg: FakeMessage = {
      ID: this.nextId.message++,
      ConversationID: conversationID,
      Role: role,
      Content: content,
      Attachments: attachments,
      ToolCalls: toolCalls,
      ToolCallID: toolCallID,
      CreatedAt: this.timestamp(),
    }
    this.messages.push(msg)
    conv.UpdatedAt = this.timestamp()
    if (role === 'user' && conv.Title === NEW_CHAT_TITLE) {
      const title = titleFromFirstMessage(content)
      if (title) conv.Title = title
    }
    return msg
  }

  /** Deletes the message at `index` and every later one. */
  deleteMessagesFrom(conversationID: number, index: number): void {
    const doomed = new Set(this.getMessages(conversationID).slice(index).map(m => m.ID))
    this.messages = this.messages.filter(m => !doomed.has(m.ID))
  }

  /** Case-insensitive substring match on content, grouped by conversation (SQLite LIKE). */
  searchMessages(query: string): Record<number, FakeMessage[]> {
    const q = query.toLowerCase()
    const out: Record<number, FakeMessage[]> = {}
    for (const m of this.messages) {
      if (!m.Content.toLowerCase().includes(q)) continue
      ;(out[m.ConversationID] ??= []).push(m)
    }
    return out
  }

  // ----------------------------------------------------------------- memories

  /** Importance first, then most recently updated, like GetMemories. */
  listMemories(): MemoryRow[] {
    return [...this.memories].sort((a, b) => b.Importance - a.Importance || b.UpdatedAt.localeCompare(a.UpdatedAt))
  }

  /** INSERT OR REPLACE keyed by `key`: the row is replaced, so it gets a new ID. */
  saveMemory(key: string, value: string, category: string, importance = 3, source = 'manual'): void {
    this.memories = this.memories.filter(m => m.Key !== key)
    this.memories.push({
      ID: this.nextId.memory++,
      Key: key,
      Value: value,
      Category: category || 'general',
      Importance: importance,
      Source: source,
      UpdatedAt: this.timestamp(),
    })
  }

  forgetMemory(id: number): void {
    this.memories = this.memories.filter(m => m.ID !== id)
  }

  // --------------------------------------------------------------- llm details

  /** One row per conversation: the newest payload replaces the previous one. */
  addLLMDetails(conversationID: number, model: string, requestCount: number, messagesJSON: string): void {
    this.llmDetails = this.llmDetails.filter(d => d.ConversationID !== conversationID)
    this.llmDetails.push({
      ID: this.nextId.details++,
      ConversationID: conversationID,
      CreatedAt: this.timestamp(),
      RequestCount: requestCount,
      Model: model,
      Messages: messagesJSON,
    })
  }

  listLLMDetails() {
    return [...this.llmDetails]
      .sort((a, b) => b.ID - a.ID)
      .map(d => ({ ...d, ConversationTitle: this.getConversation(d.ConversationID)?.Title ?? '' }))
  }

  // ------------------------------------------------------------------ seeding

  seedConversations(seed: FakeSeed['conversations'] = []): void {
    // Oldest first so IDs grow with recency, as they would have in real use.
    const ordered = [...seed].sort((a, b) => (b.daysAgo ?? 0) - (a.daysAgo ?? 0))
    for (const sc of ordered) {
      const conv = this.createConversation()
      for (const m of sc.messages) this.addSeedMessage(conv.ID, m)
      conv.Title = sc.title
      const stamp = this.timestamp(-(sc.daysAgo ?? 0) * 86_400_000)
      conv.CreatedAt = stamp
      conv.UpdatedAt = stamp
      conv.requestCount = sc.requestCount ?? 0
      conv.requestTokens = sc.requestTokens ?? 0
    }
  }

  private addSeedMessage(conversationID: number, m: SeedMessage): void {
    const toolCalls = m.toolCalls?.length
      ? JSON.stringify(
          m.toolCalls.map((tc, i) => ({ name: tc.name, argsJSON: JSON.stringify(tc.args), id: tc.id ?? `call_seed_${i}` })),
        )
      : ''
    const attachments = m.attachments?.length ? JSON.stringify(m.attachments) : ''
    this.addMessage(conversationID, m.role, m.content, attachments, toolCalls, m.toolCallId ?? '')
  }
}
