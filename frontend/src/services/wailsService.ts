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
// Conversation service
// ---------------------------------------------------------------------------
export const conversationService = {
  getAll: (): Promise<main.Conversation[]> =>
    useDevStub ? Promise.resolve([]) : GetConversations(),
  getMessages: (id: number): Promise<main.Message[]> =>
    useDevStub ? Promise.resolve([]) : GetMessages(id),
  create: (): Promise<main.Conversation> =>
    useDevStub ? Promise.resolve(stubConversation()) : CreateConversation(),
  delete: (id: number): Promise<void> =>
    useDevStub ? Promise.resolve() : DeleteConversation(id),
  search: (query: string): Promise<Record<number, main.Message[]>> =>
    useDevStub ? Promise.resolve({}) : SearchMessages(query),
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
