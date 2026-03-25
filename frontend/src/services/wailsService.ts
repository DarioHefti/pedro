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
  DeleteConversation,
  SearchMessages,
  SendMessage,
  SendMessageWithImages,
  RegenerateMessage,
  AbortMessage,
  GetSettings,
  SaveSettings,
  SetSetting,
  TestConnection,
  SignIn,
  SignOut,
  IsAuthenticated,
  SelectFile,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { main } from '../../wailsjs/go/models'

// Re-export generated types so the rest of the app never imports from wailsjs.
export type Conversation = main.Conversation
export type Message = main.Message

function hasWailsBridge(): boolean {
  if (typeof window === 'undefined') return false
  const w = window as Window & { go?: { main?: { App?: unknown } } }
  return w.go?.main?.App !== undefined
}

const useDevStub =
  import.meta.env.DEV && typeof import.meta.env !== 'undefined' && !hasWailsBridge()

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
export const MOCK_EMPTY_CHAT_UI = false

/** Reserved ID for the in-memory-only sample conversation (not stored in DB). */
export const MOCK_UI_CONVERSATION_ID = -9_001

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
export const mockEmptyChatToolCalls: { name: string; argsJSON: string }[] = [
  { name: 'websearch', argsJSON: JSON.stringify({ query: 'Wails v2 desktop Go bindings' }) },
  { name: 'grep', argsJSON: JSON.stringify({ pattern: 'wails', path: 'frontend' }) },
]

export function isSeededEmptyChatMock(msgs: main.Message[]): boolean {
  if (!MOCK_EMPTY_CHAT_UI || msgs.length !== 2) return false
  return (
    msgs[0]?.Role === 'user' &&
    msgs[0]?.Content === MOCK_SAMPLE_USER_CONTENT &&
    msgs[1]?.Role === 'assistant' &&
    msgs[1]?.Content === MOCK_SAMPLE_ASSISTANT_CONTENT
  )
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
  send: (convID: number, content: string): Promise<string> =>
    useDevStub ? Promise.resolve('') : SendMessage(convID, content),
  sendWithImages: (
    convID: number,
    content: string,
    images: string[],
  ): Promise<string> =>
    useDevStub ? Promise.resolve('') : SendMessageWithImages(convID, content, images),
  regenerate: (convID: number): Promise<string> =>
    useDevStub ? Promise.resolve('') : RegenerateMessage(convID),
  abort: (): Promise<void> => (useDevStub ? Promise.resolve() : AbortMessage()),
}

// ---------------------------------------------------------------------------
// Settings service
// ---------------------------------------------------------------------------
export const settingsService = {
  get: (): Promise<Record<string, string>> =>
    useDevStub ? Promise.resolve({}) : GetSettings(),
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
// File service
// ---------------------------------------------------------------------------
export const fileService = {
  select: (): Promise<string> => (useDevStub ? Promise.resolve('') : SelectFile()),
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
