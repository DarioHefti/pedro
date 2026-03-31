/**
 * Central service layer wrapping all Wails bridge calls.
 *
 * Wails v3 uses generated bindings in frontend/bindings/
 * Events use @wailsio/runtime Events.On/Events.Emit
 */
import { Events, Call } from '@wailsio/runtime'

// Conversation type
export type Conversation = {
  ID: number
  Title: string
  CreatedAt: string
  UpdatedAt: string
}

// Message type
export type Message = {
  ID: number
  ConversationID: number
  Role: string
  Content: string
  Attachments?: string
  ToolCalls?: string
  CreatedAt: string
}

// Persona type
export type Persona = {
  ID: number
  Name: string
  Prompt: string
  CreatedAt: string
  UpdatedAt: string
}

function hasWailsBridge(): boolean {
  if (typeof window === 'undefined') return false
  // Wails v3 injects window._wails (not window.wails which was v2)
  const w = window as Window & { _wails?: unknown }
  return w._wails !== undefined
}

const useDevStub =
  import.meta.env.DEV && typeof import.meta.env !== 'undefined' && !hasWailsBridge()

export const isWailsDevStub = useDevStub

function stubConversation(): Conversation {
  return {
    ID: Date.now(),
    Title: 'New Chat',
    CreatedAt: new Date().toISOString(),
    UpdatedAt: new Date().toISOString(),
  }
}

const MOCK_EMPTY_CHAT_UI = false
const MOCK_UI_CONVERSATION_ID = -9_001

const MOCK_SAMPLE_TITLE = 'Sample chat'
const MOCK_SAMPLE_USER_CONTENT = 'Show me a mermaid diagram for the auth flow'
const MOCK_SAMPLE_ASSISTANT_CONTENT = `This app is built with Wails: the UI is the Vite/React frontend while Go hosts native APIs.`

function buildMockConversation(): Conversation {
  const now = new Date().toISOString()
  return {
    ID: MOCK_UI_CONVERSATION_ID,
    Title: MOCK_SAMPLE_TITLE,
    CreatedAt: now,
    UpdatedAt: now,
  }
}

function buildMockMessagesForConversation(conversationID: number): Message[] {
  const now = new Date().toISOString()
  return [
    { ID: -1, ConversationID: conversationID, Role: 'user', Content: MOCK_SAMPLE_USER_CONTENT, CreatedAt: now },
    { ID: -2, ConversationID: conversationID, Role: 'assistant', Content: MOCK_SAMPLE_ASSISTANT_CONTENT, CreatedAt: now },
  ]
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
}

// ---------------------------------------------------------------------------
// Helper: Call Go method using Wails v3 Call API
// ---------------------------------------------------------------------------
async function callGo<T>(method: string, ...args: unknown[]): Promise<T> {
  const result = await Call.ByName(`main.App.${method}`, ...args)
  return result as T
}

// ---------------------------------------------------------------------------
// Conversation service
// ---------------------------------------------------------------------------
export const conversationService = {
  getAll: async (): Promise<Conversation[]> => {
    const raw = useDevStub ? [] : await callGo<Conversation[]>('GetConversations')
    const list = raw ?? []
    if (!MOCK_EMPTY_CHAT_UI) return list
    return [buildMockConversation(), ...list]
  },
  getMessages: async (id: number): Promise<Message[]> => {
    if (MOCK_EMPTY_CHAT_UI && id === MOCK_UI_CONVERSATION_ID) {
      return buildMockMessagesForConversation(MOCK_UI_CONVERSATION_ID)
    }
    const raw = useDevStub ? [] : await callGo<Message[]>('GetMessages', id)
    return raw ?? []
  },
  create: async (): Promise<Conversation> =>
    useDevStub ? stubConversation() : await callGo<Conversation>('CreateConversation'),
  delete: async (id: number): Promise<void> => {
    if (!useDevStub) {
      await callGo('DeleteConversation', id)
    }
  },
  deleteAll: async (): Promise<void> => {
    if (!useDevStub) {
      await callGo('DeleteAllConversations')
    }
  },
  search: async (query: string): Promise<Record<number, Message[]>> => {
    const base = useDevStub ? {} : await callGo<Record<number, Message[]>>('SearchMessages', query)
    return base ?? {}
  },
}

// ---------------------------------------------------------------------------
// Message service
// ---------------------------------------------------------------------------
export const messageService = {
  send: async (convID: number, content: string, selectedPersonaID: string, attachmentsJSON: string): Promise<string> =>
    useDevStub ? '' : await callGo<string>('SendMessage', convID, content, selectedPersonaID, attachmentsJSON),
  sendWithImages: async (
    convID: number,
    content: string,
    images: string[],
    selectedPersonaID: string,
    attachmentsJSON: string,
  ): Promise<string> =>
    useDevStub ? '' : await callGo<string>('SendMessageWithImages', convID, content, images, selectedPersonaID, attachmentsJSON),
  regenerate: async (convID: number, messageIndex: number, selectedPersonaID: string): Promise<string> =>
    useDevStub ? '' : await callGo<string>('RegenerateMessage', convID, messageIndex, selectedPersonaID),
  abort: async (): Promise<void> => {
    if (!useDevStub) {
      await callGo('AbortMessage')
    }
  },
  deleteMessage: async (convID: number, messageIndex: number): Promise<void> => {
    if (!useDevStub) {
      await callGo('DeleteMessage', convID, messageIndex)
    }
  },
}

// ---------------------------------------------------------------------------
// Settings service
// ---------------------------------------------------------------------------
export const settingsService = {
  get: async (): Promise<Record<string, string>> =>
    useDevStub ? {} : await callGo<Record<string, string>>('GetSettings'),
  getDefaultSystemPrompt: async (): Promise<string> =>
    useDevStub ? '' : await callGo<string>('GetDefaultSystemPrompt'),
  save: async (settings: Record<string, string>): Promise<void> => {
    if (!useDevStub) {
      await callGo('SaveSettings', settings)
    }
  },
  setSetting: async (key: string, value: string): Promise<void> => {
    if (!useDevStub) {
      await callGo('SetSetting', key, value)
    }
  },
  test: async (): Promise<string> => (useDevStub ? '' : await callGo<string>('TestConnection')),
  signIn: async (): Promise<string> => (useDevStub ? '' : await callGo<string>('SignIn')),
  signOut: async (): Promise<void> => {
    if (!useDevStub) {
      await callGo('SignOut')
    }
  },
  isAuthenticated: async (): Promise<boolean> =>
    useDevStub ? false : await callGo<boolean>('IsAuthenticated'),
}

// ---------------------------------------------------------------------------
// Personas (SQLite-backed)
// ---------------------------------------------------------------------------
export const personaService = {
  getAll: async (): Promise<Persona[]> => {
    const raw = useDevStub ? [] : await callGo<Persona[]>('GetPersonas')
    return raw ?? []
  },
  create: async (name: string, prompt: string): Promise<Persona> =>
    useDevStub
      ? { ID: Date.now(), Name: name, Prompt: prompt, CreatedAt: new Date().toISOString(), UpdatedAt: new Date().toISOString() }
      : await callGo<Persona>('CreatePersona', name, prompt),
  update: async (id: number, name: string, prompt: string): Promise<void> => {
    if (!useDevStub) {
      await callGo('UpdatePersona', id, name, prompt)
    }
  },
  delete: async (id: number): Promise<void> => {
    if (!useDevStub) {
      await callGo('DeletePersona', id)
    }
  },
  getActiveId: async (): Promise<string> =>
    useDevStub ? '' : await callGo<string>('GetActivePersonaID'),
  setActive: async (id: string): Promise<void> => {
    if (!useDevStub) {
      await callGo('SetActivePersonaID', id)
    }
  },
}

// ---------------------------------------------------------------------------
// File service
// ---------------------------------------------------------------------------
export const fileService = {
  selectFile: async (): Promise<string> => (useDevStub ? '' : await callGo<string>('SelectFile')),
  selectFolder: async (): Promise<string> => (useDevStub ? '' : await callGo<string>('SelectFolder')),
  openPath: async (path: string): Promise<string> =>
    useDevStub ? '' : await callGo<string>('OpenPath', path),
}

// ---------------------------------------------------------------------------
// Event service (streaming)
// ---------------------------------------------------------------------------
const stubEventService = {
  on: (_eventName: string, _callback: (...data: unknown[]) => void) => () => {},
  off: (_eventName: string) => {},
}

export const eventService = useDevStub
  ? stubEventService
  : {
      on: (eventName: string, callback: (...data: unknown[]) => void) => {
        Events.On(eventName, (event: { data: unknown[] }) => {
          callback(...(event.data || []))
        })
        return () => Events.Off(eventName)
      },
      off: (eventName: string) => {
        Events.Off(eventName)
      },
    }