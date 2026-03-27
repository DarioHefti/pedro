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
  SearchMessages,
  SendMessage,
  SendMessageWithImages,
  RegenerateMessage,
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
  GetPersonas,
  CreatePersona,
  UpdatePersona,
  DeletePersona,
  GetActivePersonaID,
  SetActivePersonaID,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { main } from '../../wailsjs/go/models'

// Re-export generated types so the rest of the app never imports from wailsjs.
export type Conversation = main.Conversation
export type Message = main.Message
export type Persona = main.Persona

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
  'What does this project use for the desktop shell?'

const MOCK_SAMPLE_ASSISTANT_CONTENT =
  'This app is built with Wails: the UI is the Vite/React frontend while Go hosts native APIs. Tool calls like search and repo grep are typical for how the assistant answers.'

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
    msgs[1]?.Content === MOCK_SAMPLE_ASSISTANT_CONTENT
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
  /** selectedPersonaID: DB row id; backend loads prompt text from SQLite. */
  send: (convID: number, content: string, selectedPersonaID: string, attachmentsJSON: string): Promise<string> =>
    useDevStub ? Promise.resolve('') : SendMessage(convID, content, selectedPersonaID, attachmentsJSON),
  sendWithImages: (
    convID: number,
    content: string,
    images: string[],
    selectedPersonaID: string,
    attachmentsJSON: string,
  ): Promise<string> =>
    useDevStub
      ? Promise.resolve('')
      : SendMessageWithImages(convID, content, images, selectedPersonaID, attachmentsJSON),
  regenerate: (convID: number, messageIndex: number, selectedPersonaID: string): Promise<string> =>
    useDevStub ? Promise.resolve('') : RegenerateMessage(convID, messageIndex, selectedPersonaID),
  abort: (): Promise<void> => (useDevStub ? Promise.resolve() : AbortMessage()),
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
// Personas (SQLite-backed)
// ---------------------------------------------------------------------------
export const personaService = {
  getAll: async (): Promise<main.Persona[]> => {
    const raw = useDevStub ? [] : await GetPersonas()
    return raw ?? []
  },
  create: (name: string, prompt: string): Promise<main.Persona> =>
    useDevStub
      ? Promise.resolve(
          new main.Persona({
            ID: Date.now(),
            Name: name,
            Prompt: prompt,
            CreatedAt: new Date().toISOString(),
            UpdatedAt: new Date().toISOString(),
          }),
        )
      : CreatePersona(name, prompt),
  update: (id: number, name: string, prompt: string): Promise<void> =>
    useDevStub ? Promise.resolve() : UpdatePersona(id, name, prompt),
  delete: (id: number): Promise<void> =>
    useDevStub ? Promise.resolve() : DeletePersona(id),
  getActiveId: (): Promise<string> =>
    useDevStub ? Promise.resolve('') : GetActivePersonaID(),
  setActive: (id: string): Promise<void> =>
    useDevStub ? Promise.resolve() : SetActivePersonaID(id),
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
