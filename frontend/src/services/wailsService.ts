/**
 * Central service layer wrapping all Wails bridge calls.
 *
 * This is the ONLY place in the frontend that imports from the auto-generated
 * wailsjs directory. All components and hooks must go through these services
 * so that the Wails coupling is isolated to a single, swappable boundary.
 *
 * In Vite dev, opening the app in a normal browser (e.g. Playwright) has no
 * `window.go` bridge; we return safe stubs so the UI still renders.
 */
import {
  GetConversations,
  GetMessages,
  CreateConversation,
  DeleteAllConversations,
  DeleteConversation,
  DeleteMessage,
  SearchMessages,
  SendMessage,
  SendMessageWithImages,
  RegenerateMessage,
  ResendMessage,
  AbortMessage,
  GetSettings,
  GetDefaultSystemPrompt,
  SaveSettings,
  SetSetting,
  TestConnection,
  SignIn,
  SignOut,
  IsAuthenticated,
  SelectFile,
  SelectFolder,
  OpenPath,
  SaveFile,
  SaveBase64File,
  SaveImage,
  GetMemories,
  SaveMemory,
  ForgetMemory,
  GetRequestCounts,
  GetGlobalRequestCount,
  GetLifetimeStats,
  GetLLMDetails,
  ClearLLMDetails,
} from '../../wailsjs/go/main/App'
import {
  CheckForUpdate,
  DownloadAndInstall,
  GetCurrentVersion,
  GetWindowsDefenderExclusion,
} from '../../wailsjs/go/main/Updater'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { main, shared } from '../../wailsjs/go/models'

// Re-export generated types so the rest of the app never imports from wailsjs.
export type Conversation = main.Conversation
export type Message = main.Message
export type MemoryRecord = shared.MemoryRecord
export type UpdateInfo = main.UpdateInfo
export type LLMDetailsEntry = main.LLMDetailsEntry
export type SaveResult = Pick<main.SaveResult, 'path' | 'cancelled'>

function hasWailsBridge(): boolean {
  if (typeof window === 'undefined') return false
  const w = window as Window & { go?: { main?: { App?: unknown } } }
  return w.go?.main?.App !== undefined
}

const useDevStub =
  import.meta.env.DEV && typeof import.meta.env !== 'undefined' && !hasWailsBridge()

/** True when the app runs without the Wails bridge (e.g. Vite in a normal browser). */
export const isWailsDevStub = useDevStub

function stubConversation(): main.Conversation {
  return new main.Conversation({
    ID: Date.now(),
    Title: 'New Chat',
    CreatedAt: new Date().toISOString(),
    UpdatedAt: new Date().toISOString(),
  })
}

// ---------------------------------------------------------------------------
// UI dev: virtual conversation (sidebar row + thread). Set MOCK_EMPTY_CHAT_UI
// false (or delete this block) to remove. Does not touch the Go backend.
// ---------------------------------------------------------------------------

/** Dev-only sample thread in sidebar. Keep `false` for production builds. */
const MOCK_EMPTY_CHAT_UI = false

/** Reserved ID for the in-memory-only sample conversation (not stored in DB). */
const MOCK_UI_CONVERSATION_ID = -9_001

const MOCK_SAMPLE_TITLE = 'Sample chat'

const MOCK_SAMPLE_USER_CONTENT =
  'Show me a mermaid diagram for the auth flow'

const MOCK_SAMPLE_ASSISTANT_CONTENT =
  `This app is built with Wails: the UI is the Vite/React frontend while Go hosts native APIs. Here is a diagram showing the flow:

\`\`\`mermaid
flowchart TD
    A[User Signs Up] --> B[Derive KEK from Password]
    B --> C[Generate random DEK]
    C --> D[Wrap DEK with KEK]
    D --> E[Generate Recovery Key]
    E --> F[Wrap DEK with Recovery Key]
    F --> G[Encrypt Data with DEK]
    G --> H[Upload to Server]
    
    style A fill:#bbf
    style H fill:#fbb
\`\`\`

I used web search to find the latest Wails v2 documentation and searched the codebase for existing auth implementations to ensure consistency.`

function buildMockConversation(): main.Conversation {
  const now = new Date().toISOString()
  return new main.Conversation({
    ID: MOCK_UI_CONVERSATION_ID,
    Title: MOCK_SAMPLE_TITLE,
    CreatedAt: now,
    UpdatedAt: now,
  })
}

function buildMockMessagesForConversation(conversationID: number): main.Message[] {
  const now = new Date().toISOString()
  const mockToolCallsJSON = JSON.stringify(mockEmptyChatToolCalls)
  return [
    new main.Message({
      ID: -1,
      ConversationID: conversationID,
      Role: 'user',
      Content: MOCK_SAMPLE_USER_CONTENT,
      CreatedAt: now,
    }),
    new main.Message({
      ID: -2,
      ConversationID: conversationID,
      Role: 'assistant',
      Content: MOCK_SAMPLE_ASSISTANT_CONTENT,
      ToolCalls: mockToolCallsJSON,
      CreatedAt: now,
    }),
  ]
}

/** Sample tool rows paired with `isSeededEmptyChatMock` for the mock thread. */
const mockEmptyChatToolCalls: { name: string; argsJSON: string }[] = [
  { name: 'websearch', argsJSON: JSON.stringify({ query: 'Wails v2 desktop Go bindings' }) },
  { name: 'grep', argsJSON: JSON.stringify({ pattern: 'wails', path: 'frontend' }) },
]

function isSeededEmptyChatMock(msgs: main.Message[]): boolean {
  if (!MOCK_EMPTY_CHAT_UI || msgs.length !== 2) return false
  return (
    msgs[0]?.Role === 'user' &&
    msgs[0]?.Content === MOCK_SAMPLE_USER_CONTENT &&
    msgs[1]?.Role === 'assistant' &&
    msgs[1]?.Content?.includes('I used web search')
  )
}

export const uiConversationService = {
  initialConversationID(): number | null {
    return MOCK_EMPTY_CHAT_UI ? MOCK_UI_CONVERSATION_ID : null
  },
  isVirtualConversation(id: number | null): boolean {
    return MOCK_EMPTY_CHAT_UI && id === MOCK_UI_CONVERSATION_ID
  },
  canDeleteConversation(id: number): boolean {
    return !this.isVirtualConversation(id)
  },
  getMockToolCallsForMessages(
    msgs: main.Message[],
  ): { name: string; argsJSON: string }[] {
    return isSeededEmptyChatMock(msgs)
      ? mockEmptyChatToolCalls.map(tc => ({ ...tc }))
      : []
  },
}

// ---------------------------------------------------------------------------
// Conversation service
// ---------------------------------------------------------------------------
export const conversationService = {
  getAll: async (): Promise<main.Conversation[]> => {
    const raw = useDevStub ? [] : await GetConversations()
    const list = raw ?? []
    if (!MOCK_EMPTY_CHAT_UI) return list
    return [buildMockConversation(), ...list]
  },
  getMessages: async (id: number): Promise<main.Message[]> => {
    if (MOCK_EMPTY_CHAT_UI && id === MOCK_UI_CONVERSATION_ID) {
      return buildMockMessagesForConversation(MOCK_UI_CONVERSATION_ID)
    }
    const raw = useDevStub ? [] : await GetMessages(id)
    return raw ?? []
  },
  create: (): Promise<main.Conversation> =>
    useDevStub ? Promise.resolve(stubConversation()) : CreateConversation(),
  delete: (id: number): Promise<void> => {
    if (MOCK_EMPTY_CHAT_UI && id === MOCK_UI_CONVERSATION_ID) {
      return Promise.resolve()
    }
    return useDevStub ? Promise.resolve() : DeleteConversation(id)
  },
  /** Removes every stored conversation (and messages). UI-only mock rows are not in the DB. */
  deleteAll: (): Promise<void> =>
    useDevStub ? Promise.resolve() : DeleteAllConversations(),
  search: async (query: string): Promise<Record<number, main.Message[]>> => {
    const base = useDevStub ? {} : await SearchMessages(query)
    const out: Record<number, main.Message[]> = { ...(base ?? {}) }
    if (MOCK_EMPTY_CHAT_UI && query.trim()) {
      const q = query.trim().toLowerCase()
      const mockMsgs = buildMockMessagesForConversation(MOCK_UI_CONVERSATION_ID)
      if (MOCK_SAMPLE_TITLE.toLowerCase().includes(q)) {
        out[MOCK_UI_CONVERSATION_ID] = mockMsgs
      } else {
        const matching = mockMsgs.filter(m =>
          (m.Content || '').toLowerCase().includes(q),
        )
        if (matching.length > 0) {
          out[MOCK_UI_CONVERSATION_ID] = matching
        }
      }
    }
    return out
  },
}

// ---------------------------------------------------------------------------
// Message service
// ---------------------------------------------------------------------------
export const messageService = {
  send: (convID: number, content: string, attachmentsJSON: string): Promise<string> =>
    useDevStub ? Promise.resolve('') : SendMessage(convID, content, attachmentsJSON),
  sendWithImages: (convID: number, content: string, images: string[], attachmentsJSON: string): Promise<string> =>
    useDevStub ? Promise.resolve('') : SendMessageWithImages(convID, content, images, attachmentsJSON),
  regenerate: (convID: number, messageIndex: number): Promise<string> =>
    useDevStub ? Promise.resolve('') : RegenerateMessage(convID, messageIndex),
  resend: (convID: number, messageIndex: number): Promise<string> =>
    useDevStub ? Promise.resolve('') : ResendMessage(convID, messageIndex),
  abort: (): Promise<void> => (useDevStub ? Promise.resolve() : AbortMessage()),
  deleteMessage: (convID: number, messageIndex: number): Promise<void> =>
    useDevStub ? Promise.resolve() : DeleteMessage(convID, messageIndex),
}

// ---------------------------------------------------------------------------
// Settings service
// ---------------------------------------------------------------------------
export const settingsService = {
  get: (): Promise<Record<string, string>> =>
    useDevStub ? Promise.resolve({}) : GetSettings(),
  getDefaultSystemPrompt: (): Promise<string> =>
    useDevStub ? Promise.resolve('') : GetDefaultSystemPrompt(),
  save: (settings: Record<string, string>): Promise<void> =>
    useDevStub ? Promise.resolve() : SaveSettings(settings),
  setSetting: (key: string, value: string): Promise<void> =>
    useDevStub ? Promise.resolve() : SetSetting(key, value),
  test: (): Promise<string> => (useDevStub ? Promise.resolve('') : TestConnection()),
  signIn: (): Promise<string> => (useDevStub ? Promise.resolve('') : SignIn()),
  signOut: (): Promise<void> => (useDevStub ? Promise.resolve() : SignOut()),
  isAuthenticated: (): Promise<boolean> =>
    useDevStub ? Promise.resolve(false) : IsAuthenticated(),
}

// ---------------------------------------------------------------------------
// Memory (SQLite-backed)
// ---------------------------------------------------------------------------
export const memoryService = {
  getAll: async (): Promise<shared.MemoryRecord[]> => {
    const raw = useDevStub ? [] : await GetMemories()
    return raw ?? []
  },
  save: (key: string, value: string, category: string): Promise<void> =>
    useDevStub ? Promise.resolve() : SaveMemory(key, value, category),
  delete: (id: number): Promise<void> =>
    useDevStub ? Promise.resolve() : ForgetMemory(id),
}

// ---------------------------------------------------------------------------
// File service
// ---------------------------------------------------------------------------
export const fileService = {
  selectFile: (): Promise<string> => (useDevStub ? Promise.resolve('') : SelectFile()),
  selectFolder: (): Promise<string> => (useDevStub ? Promise.resolve('') : SelectFolder()),
  /** Opens with the OS default app (WebView blocks file:// links). */
  openPath: (path: string): Promise<string> =>
    useDevStub ? Promise.resolve('') : OpenPath(path),
  /** Asks where to save text content (native save dialog) and writes it there. */
  saveFile: (defaultFilename: string, content: string): Promise<SaveResult> =>
    useDevStub
      ? browserDownload(defaultFilename, new Blob([content], { type: 'text/plain' }))
      : SaveFile(defaultFilename, content),
  /** Asks where to save binary content, given as base64, and writes it there. */
  saveBase64File: (defaultFilename: string, base64: string): Promise<SaveResult> =>
    useDevStub
      ? fetch(`data:application/octet-stream;base64,${base64}`)
          .then(res => res.blob())
          .then(blob => browserDownload(defaultFilename, blob))
      : SaveBase64File(defaultFilename, base64),
  /** Asks where to save an image the chat shows (stored image URL or data URL) and copies it there. */
  saveImage: (src: string): Promise<SaveResult> =>
    useDevStub
      ? fetch(src)
          .then(res => res.blob())
          .then(blob => browserDownload(src.startsWith('data:') ? 'image.png' : src.split('/').pop() || 'image.png', blob))
      : SaveImage(src),
}

/** Dev-only stand-in for the native save dialog: a regular browser download. */
function browserDownload(filename: string, blob: Blob): Promise<SaveResult> {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  return Promise.resolve({ path: filename, cancelled: false })
}

// ---------------------------------------------------------------------------
// Updater service
// ---------------------------------------------------------------------------
export const updaterService = {
  checkForUpdate: (): Promise<main.UpdateInfo> =>
    useDevStub
      ? Promise.resolve(new main.UpdateInfo({ available: false, currentVersion: 'dev', latestVersion: 'dev', releaseURL: '', assetName: '', assetURL: '' }))
      : CheckForUpdate(),
  downloadAndInstall: (assetURL: string, assetName: string): Promise<void> =>
    useDevStub ? Promise.resolve() : DownloadAndInstall(assetURL, assetName),
  getCurrentVersion: (): Promise<string> =>
    useDevStub ? Promise.resolve('dev') : GetCurrentVersion(),
  getWindowsDefenderExclusion: (): Promise<string> =>
    useDevStub ? Promise.resolve('') : GetWindowsDefenderExclusion(),
}

// ---------------------------------------------------------------------------
// Request stats service (LLM request counters + token estimates)
// ---------------------------------------------------------------------------
export const statsService = {
  getRequestCounts: (conversationID: number): Promise<main.RequestCounts> =>
    useDevStub ? Promise.resolve(new main.RequestCounts({ PerChat: 0, Global: 0 })) : GetRequestCounts(conversationID),
  getGlobalRequestCount: (): Promise<number> =>
    useDevStub ? Promise.resolve(0) : GetGlobalRequestCount(),
  getLifetimeStats: (): Promise<main.LifetimeStats> =>
    useDevStub
      ? Promise.resolve(new main.LifetimeStats({ totalRequests: 0, totalTokens: 0 }))
      : GetLifetimeStats(),
}

// ---------------------------------------------------------------------------
// LLM request details (final payloads sent to the provider)
// ---------------------------------------------------------------------------
export const detailsService = {
  getAll: (): Promise<main.LLMDetailsEntry[]> =>
    useDevStub ? Promise.resolve([]) : GetLLMDetails(),
  clear: (): Promise<void> =>
    useDevStub ? Promise.resolve() : ClearLLMDetails(),
}

// ---------------------------------------------------------------------------
// Event service (streaming)
// ---------------------------------------------------------------------------
const stubEventService = {
  on: (_eventName: string, _callback: (...data: unknown[]) => void) => () => {},
  off: (_eventName: string, ..._additional: string[]) => {},
}

export const eventService = useDevStub
  ? stubEventService
  : {
      on: EventsOn,
      off: EventsOff,
    }
