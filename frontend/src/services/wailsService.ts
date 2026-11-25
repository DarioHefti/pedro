/**
 * Central service layer wrapping all Wails bridge calls.
 *
 * This is the ONLY place in the frontend that imports from the auto-generated
 * wailsjs directory. All components and hooks must go through these services
 * so that the Wails coupling is isolated to a single, swappable boundary.
 *
 * Opened in a normal browser without the Go bridge (plain `vite` dev server), every call
 * resolves to a harmless stub so the UI still renders. For a working backend in the browser
 * use `npm run dev:mock`, which installs the fake backend from e2e/fake-backend.
 */
import {
  AbortMessage,
  ClearLLMDetails,
  CreateConversation,
  DeleteAllConversations,
  DeleteConversation,
  DeleteMessagesFrom,
  ForgetMemory,
  GetConversations,
  GetDefaultSystemPrompt,
  GetGlobalRequestCount,
  GetLLMDetails,
  GetLifetimeStats,
  GetMemories,
  GetMessages,
  GetRequestCounts,
  GetSettings,
  IsAuthenticated,
  OpenPath,
  RegenerateMessage,
  ResendMessage,
  SaveBase64File,
  SaveFile,
  SaveImage,
  SaveMemory,
  SaveSettings,
  SearchMessages,
  SelectFile,
  SelectFolder,
  SendMessage,
  SendMessageWithImages,
  SetSetting,
  SignIn,
  SignOut,
  TestConnection,
} from '../../wailsjs/go/main/App'
import {
  CheckForUpdate,
  DownloadAndInstall,
  GetCurrentVersion,
  GetWindowsDefenderExclusion,
} from '../../wailsjs/go/main/Updater'
import { BrowserOpenURL, EventsOff, EventsOn, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'
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

const useDevStub = import.meta.env.DEV && !hasWailsBridge()

/** True when the app runs without the Wails bridge (e.g. Vite in a normal browser). */
export const isWailsDevStub = useDevStub

type AsyncFn = (...args: never[]) => Promise<unknown>

/** The bridge call, or — without a bridge — a stand-in with the same signature. */
function bind<F extends AsyncFn>(real: F, stub: F): F {
  return useDevStub ? stub : real
}

/** A stand-in that resolves to a fixed value. */
function resolves<F extends AsyncFn>(value: Awaited<ReturnType<F>>): F {
  return (() => Promise.resolve(value)) as unknown as F
}

// ---------------------------------------------------------------------------
// Conversations and messages
// ---------------------------------------------------------------------------
export const conversationService = {
  getAll: async (): Promise<Conversation[]> => (await bind(GetConversations, resolves([]))()) ?? [],
  getMessages: async (id: number): Promise<Message[]> => (await bind(GetMessages, resolves([]))(id)) ?? [],
  create: bind(CreateConversation, () =>
    Promise.resolve(
      new main.Conversation({ ID: Date.now(), Title: 'New Chat', CreatedAt: new Date().toISOString(), UpdatedAt: new Date().toISOString() }),
    ),
  ),
  delete: bind(DeleteConversation, resolves(undefined)),
  /** Removes every stored conversation (and messages). */
  deleteAll: bind(DeleteAllConversations, resolves(undefined)),
  /** Map of conversation ID → messages whose content contains the query. */
  search: async (query: string): Promise<Record<number, Message[]>> => (await bind(SearchMessages, resolves({}))(query)) ?? {},
}

export const messageService = {
  send: bind(SendMessage, resolves('')),
  sendWithImages: bind(SendMessageWithImages, resolves('')),
  regenerate: bind(RegenerateMessage, resolves('')),
  resend: bind(ResendMessage, resolves('')),
  abort: bind(AbortMessage, resolves(undefined)),
  /** Deletes the message at `messageIndex` and every message after it. */
  deleteFrom: bind(DeleteMessagesFrom, resolves(undefined)),
}

// ---------------------------------------------------------------------------
// Settings and memory
// ---------------------------------------------------------------------------
export const settingsService = {
  get: bind(GetSettings, resolves({})),
  getDefaultSystemPrompt: bind(GetDefaultSystemPrompt, resolves('')),
  save: bind(SaveSettings, resolves(undefined)),
  setSetting: bind(SetSetting, resolves(undefined)),
  test: bind(TestConnection, resolves('')),
  signIn: bind(SignIn, resolves('')),
  signOut: bind(SignOut, resolves(undefined)),
  isAuthenticated: bind(IsAuthenticated, resolves(false)),
}

export const memoryService = {
  getAll: async (): Promise<MemoryRecord[]> => (await bind(GetMemories, resolves([]))()) ?? [],
  save: bind(SaveMemory, resolves(undefined)),
  delete: bind(ForgetMemory, resolves(undefined)),
}

// ---------------------------------------------------------------------------
// Files, links and native drops
// ---------------------------------------------------------------------------

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

const openPath = bind(OpenPath, resolves(''))

export const fileService = {
  selectFile: bind(SelectFile, resolves('')),
  selectFolder: bind(SelectFolder, resolves('')),
  /** Opens with the OS default app (WebView blocks file:// links). Resolves to an error message, or ''. */
  openPath,
  /** Opens a file or folder with the OS default app; failures are only logged. */
  open: (path: string): Promise<void> =>
    openPath(path).then(err => {
      if (err) console.warn('[OpenPath]', err)
    }),
  /** Asks where to save text content (native save dialog) and writes it there. */
  saveFile: (defaultFilename: string, content: string): Promise<SaveResult> =>
    useDevStub ? browserDownload(defaultFilename, new Blob([content], { type: 'text/plain' })) : SaveFile(defaultFilename, content),
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

export const browserService = {
  /** Opens a link in the system browser; without the bridge the anchor's default handling applies. */
  canOpenExternally: !useDevStub,
  openURL: (url: string): void => {
    if (!useDevStub) BrowserOpenURL(url)
  },
}

function hasNativeFileDrop(): boolean {
  return typeof window !== 'undefined' && !!(window as Window & { runtime?: { OnFileDrop?: unknown } }).runtime?.OnFileDrop
}

export const fileDropService = {
  /** True when the webview delivers dropped files as native paths (Wails OnFileDrop). */
  get isNative(): boolean {
    return hasNativeFileDrop()
  },
  /** Subscribes to native file drops; returns the unsubscribe function. */
  onDrop(callback: (paths: string[]) => void): () => void {
    if (!hasNativeFileDrop()) return () => {}
    OnFileDrop((_x: number, _y: number, paths: string[]) => callback(paths), false)
    return () => {
      if (hasNativeFileDrop()) OnFileDropOff()
    }
  },
}

// ---------------------------------------------------------------------------
// Updater
// ---------------------------------------------------------------------------
export const updaterService = {
  checkForUpdate: bind(
    CheckForUpdate,
    resolves(new main.UpdateInfo({ available: false, currentVersion: 'dev', latestVersion: 'dev', releaseURL: '', assetName: '', assetURL: '' })),
  ),
  downloadAndInstall: bind(DownloadAndInstall, resolves(undefined)),
  getCurrentVersion: bind(GetCurrentVersion, resolves('dev')),
  getWindowsDefenderExclusion: bind(GetWindowsDefenderExclusion, resolves('')),
}

// ---------------------------------------------------------------------------
// Request stats (LLM request counters + token estimates) and captured payloads
// ---------------------------------------------------------------------------
export const statsService = {
  getRequestCounts: bind(GetRequestCounts, resolves(new main.RequestCounts({ perChat: 0, perChatTokens: 0, global: 0 }))),
  getGlobalRequestCount: bind(GetGlobalRequestCount, resolves(0)),
  getLifetimeStats: bind(GetLifetimeStats, resolves(new main.LifetimeStats({ totalRequests: 0, totalTokens: 0 }))),
}

export const detailsService = {
  getAll: bind(GetLLMDetails, resolves([])),
  clear: bind(ClearLLMDetails, resolves(undefined)),
}

// ---------------------------------------------------------------------------
// Events (streaming, counters, updater progress)
// ---------------------------------------------------------------------------
type EventCallback = (...data: never[]) => void

export const eventService = {
  /** Subscribes to a backend event; returns the unsubscribe function. */
  on: (eventName: string, callback: EventCallback): (() => void) =>
    useDevStub ? () => {} : EventsOn(eventName, callback as (...data: unknown[]) => void),
  off: (eventName: string, ...additional: string[]): void => {
    if (!useDevStub) EventsOff(eventName, ...additional)
  },
}
