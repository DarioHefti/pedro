import type * as AppBindings from '../../wailsjs/go/main/App'
import type * as UpdaterBindings from '../../wailsjs/go/main/Updater'
import { main, shared } from '../../wailsjs/go/models'
import { FakeEventBus } from './events'
import { chunkText, defaultReply } from './replies'
import { FakeStore } from './store'
import type { FakeControl, FakeSeed, RecordedCall, ReplyScript } from './types'

/** Every Go method bound on `App`: the fake must implement them all, with the same signatures. */
export type FakeApp = typeof AppBindings
export type FakeUpdater = typeof UpdaterBindings

export const DEFAULT_SYSTEM_PROMPT =
  'You are Pedro, a helpful assistant with access to multiple tools to help the user with their request.'

/** A configured OpenAI-compatible provider, so a fresh fake is ready to chat. */
export const DEFAULT_SETTINGS: Record<string, string> = {
  provider_type: 'compat',
  compat_base_url: 'http://localhost:1234/v1',
  compat_api_key: '',
  compat_model: 'pedro-test-model',
}

const DEFAULT_USAGE = { prompt: 180, completion: 60 }
const CONNECTION_TEST_KEY = 'connection_test'
const DEFAULT_IMAGE_DEPLOYMENT = 'gpt-image-2.5-sunburst'
const DEFAULT_TRANSCRIPTION_DEPLOYMENT = 'gpt-transcribe'

/** Settings keys whose change invalidates the stored connection test (settings_connection.go). */
const CONNECTION_KEYS = new Set([
  'provider_type', 'azure_endpoint', 'azure_deployment', 'azure_tenant_id', 'azure_api_key',
  'azure_image_deployment', 'azure_transcription_deployment', 'openai_api_key', 'openai_model',
  'compat_base_url', 'compat_api_key', 'compat_model', 'system_prompt', 'custom_system_prompt',
  'welcome_message', 'design_light_base_color', 'design_dark_base_color', 'design_ui_font_size_px',
  'design_message_font_size_px',
])

/** Tool schemas included in captured request payloads, for the request details view. */
const TOOL_DEFS = [
  {
    type: 'function',
    function: {
      name: 'web_search',
      description: 'Search the web and return the top results.',
      parameters: {
        type: 'object',
        properties: { query: { type: 'string', description: 'What to search for' } },
        required: ['query'],
      },
    },
  },
  {
    type: 'function',
    function: {
      name: 'read_file',
      description: 'Read a local text file, paginated.',
      parameters: {
        type: 'object',
        properties: {
          path: { type: 'string', description: 'Absolute path' },
          offset: { type: 'integer', description: 'First line' },
          limit: { type: 'integer', description: 'Line count' },
        },
        required: ['path'],
      },
    },
  },
]

/** Port of settings_connection.go connectionSettingsFingerprint; must match the frontend's fingerprint(). */
export function connectionSettingsFingerprint(settings: Record<string, string>): string {
  const pt = settings.provider_type || 'azure'
  const rec: Record<string, string> = {
    provider_type: pt,
    system_prompt: settings.system_prompt ?? '',
    custom_system_prompt: settings.custom_system_prompt ?? '',
    welcome_message: settings.welcome_message || 'Welcome to Pedro',
    design_light_base_color: settings.design_light_base_color ?? '',
    design_dark_base_color: settings.design_dark_base_color ?? '',
    design_ui_font_size_px: settings.design_ui_font_size_px ?? '',
    design_message_font_size_px: settings.design_message_font_size_px ?? '',
  }
  if (pt === 'azure') {
    rec.azure_endpoint = settings.azure_endpoint ?? ''
    rec.azure_deployment = settings.azure_deployment ?? ''
    rec.azure_tenant_id = (settings.azure_tenant_id ?? '').trim()
  } else if (pt === 'azure_apikey') {
    rec.azure_endpoint = settings.azure_endpoint ?? ''
    rec.azure_deployment = settings.azure_deployment ?? ''
    rec.azure_api_key = settings.azure_api_key ?? ''
    rec.azure_image_deployment = settings.azure_image_deployment ?? DEFAULT_IMAGE_DEPLOYMENT
    rec.azure_transcription_deployment = settings.azure_transcription_deployment ?? DEFAULT_TRANSCRIPTION_DEPLOYMENT
  } else if (pt === 'openai') {
    rec.openai_api_key = settings.openai_api_key ?? ''
    rec.openai_model = settings.openai_model || 'gpt-4o'
  } else if (pt === 'compat') {
    rec.compat_base_url = settings.compat_base_url ?? ''
    rec.compat_api_key = settings.compat_api_key ?? ''
    rec.compat_model = settings.compat_model ?? ''
  }
  return JSON.stringify(Object.keys(rec).sort().map(k => [k, rec[k]]))
}

class Cancelled extends Error {}
class ScriptedFailure extends Error {}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(new Cancelled())
    const timer = setTimeout(resolve, ms)
    signal?.addEventListener('abort', () => {
      clearTimeout(timer)
      reject(new Cancelled())
    }, { once: true })
  })
}

/** Wails serializes every value as JSON across the bridge; so does the fake. */
function overTheWire<T>(value: T): T {
  return value === undefined ? value : (JSON.parse(JSON.stringify(value)) as T)
}

interface ToolCallRecord {
  name: string
  argsJSON: string
  id: string
}

export interface FakeBackend {
  app: FakeApp
  updater: FakeUpdater
  runtime: Record<string, unknown>
  control: FakeControl
  store: FakeStore
  events: FakeEventBus
}

/** Builds an in-memory backend that behaves like app.go + database.go + updater.go. */
export function createFakeBackend(seed: FakeSeed = {}): FakeBackend {
  const store = new FakeStore()
  const events = new FakeEventBus()
  const calls: RecordedCall[] = []
  const replies: ReplyScript[] = [...(seed.replies ?? [])]
  const fileSelections = [...(seed.fileSelections ?? [])]
  const folderSelections = [...(seed.folderSelections ?? [])]
  let fileDrop: ((x: number, y: number, paths: string[]) => void) | null = null
  let activeRun: AbortController | null = null
  let callSeq = 1
  let globalRequests = seed.stats?.globalRequests ?? 0
  let lifetimeTokens = seed.stats?.lifetimeTokens ?? 0

  store.settings = seed.unconfigured ? { ...(seed.settings ?? {}) } : { ...DEFAULT_SETTINGS, ...(seed.settings ?? {}) }
  store.seedConversations(seed.conversations)
  for (const m of seed.memories ?? []) store.saveMemory(m.key, m.value, m.category ?? '', m.importance ?? 3, 'seed')

  const llmConfigured = () => !!store.settings.provider_type
  const modelName = () =>
    store.settings.compat_model || store.settings.openai_model || store.settings.azure_deployment || 'gpt-4o'

  function recordRequest(conversationID: number, usage = DEFAULT_USAGE) {
    const conv = store.getConversation(conversationID)
    globalRequests += 1
    lifetimeTokens += usage.prompt + usage.completion
    if (conv) {
      conv.requestCount += 1
      conv.requestTokens += usage.prompt + usage.completion
    }
    events.emit(
      'request_count_updated',
      conversationID,
      conv?.requestCount ?? 0,
      globalRequests,
      conv?.requestTokens ?? 0,
      usage.prompt,
      usage.completion,
    )
  }

  function recordLLMDetails(conversationID: number, finalText: string) {
    const history = store.getMessages(conversationID).map(m => {
      const out: Record<string, unknown> = { role: m.Role, content: m.Content }
      if (m.ToolCalls) {
        const tcs = JSON.parse(m.ToolCalls) as ToolCallRecord[]
        out.tool_calls = tcs.map(tc => ({ id: tc.id, type: 'function', function: { name: tc.name, arguments: tc.argsJSON } }))
      }
      if (m.ToolCallID) out.tool_call_id = m.ToolCallID
      return out
    })
    const request = {
      model: modelName(),
      messages: [{ role: 'system', content: store.settings.system_prompt || DEFAULT_SYSTEM_PROMPT }, ...history],
      tools: TOOL_DEFS,
      temperature: 0.7,
      stream: true,
    }
    store.addLLMDetails(conversationID, modelName(), globalRequests, JSON.stringify({ request, response: finalText }))
  }

  /**
   * Mirrors App.runChat: streams the scripted reply, then persists it like the Go side does.
   * A stopped or failed reply keeps what it generated: the tool calls that ran, their results
   * and the partly streamed text.
   */
  async function runChat(conversationID: number): Promise<string> {
    const controller = new AbortController()
    activeRun = controller
    const history = store.getMessages(conversationID)
    const lastUser = [...history].reverse().find(m => m.Role === 'user')
    const script = replies.shift() ?? defaultReply(lastUser?.Content ?? '')
    const chunkSize = script.chunkSize ?? 12
    const delay = script.chunkDelayMs ?? 12
    const usage = script.usage ?? DEFAULT_USAGE
    let streamed = ''
    const toolCalls: ToolCallRecord[] = []

    /** A finished reply is always stored; a cut-off one only when some text arrived (as in Go). */
    const storeTurn = (reply: string, finished: boolean) => {
      if (toolCalls.length > 0) {
        store.addMessage(conversationID, 'assistant', '', '', JSON.stringify(toolCalls), '')
        toolCalls.forEach((tc, i) => store.addMessage(conversationID, 'tool', script.toolCalls?.[i]?.result ?? 'ok', '', '', tc.id))
      }
      if (finished || reply.trim() !== '') store.addMessage(conversationID, 'assistant', reply)
    }

    try {
      await sleep(script.firstTokenDelayMs ?? 40, controller.signal)
      for (const tc of script.toolCalls ?? []) {
        const record = { name: tc.name, argsJSON: JSON.stringify(tc.args), id: `call_${callSeq++}` }
        toolCalls.push(record)
        events.emit('tool_call', conversationID, record.name, record.argsJSON)
        await sleep(delay * 4, controller.signal)
      }
      if (toolCalls.length > 0) recordRequest(conversationID, usage)
      if (script.error) throw new ScriptedFailure(script.error)
      for (const chunk of chunkText(script.text ?? '', chunkSize)) {
        await sleep(delay, controller.signal)
        streamed += chunk
        events.emit('stream_chunk', conversationID, chunk)
      }
      recordRequest(conversationID, usage)
    } catch (err) {
      if (err instanceof Cancelled) {
        storeTurn(streamed, false)
        return streamed
      }
      if (err instanceof ScriptedFailure) {
        storeTurn(streamed, false)
        return `Error: ${err.message}`
      }
      throw err
    } finally {
      if (activeRun === controller) activeRun = null
    }

    const finalText = script.text ?? ''
    // The captured payload is the last request sent, so it holds everything but the reply itself.
    recordLLMDetails(conversationID, finalText)
    storeTurn(finalText, true)
    return finalText
  }

  async function sendMessage(conversationID: number, content: string, attachmentsJSON: string): Promise<string> {
    try {
      store.addMessage(conversationID, 'user', content, attachmentsJSON)
    } catch (err) {
      return `Error: Failed to save message: ${(err as Error).message}`
    }
    events.emit('conversation_updated', conversationID)
    if (!llmConfigured()) return 'Error: please configure LLM provider settings first'
    return runChat(conversationID)
  }

  function validIndex(conversationID: number, index: number, role: string): string | null {
    const msgs = store.getMessages(conversationID)
    if (index < 0 || index >= msgs.length) return 'Error: Invalid message index'
    if (msgs[index].Role !== role) {
      return role === 'user' ? 'Error: No user message to resend' : 'Error: No assistant message to regenerate'
    }
    return null
  }

  function setSetting(key: string, value: string) {
    store.settings[key] = value
    if (CONNECTION_KEYS.has(key)) delete store.settings[CONNECTION_TEST_KEY]
  }

  const app: FakeApp = {
    AbortMessage: async () => {
      activeRun?.abort()
    },
    ClearLLMDetails: async () => {
      store.llmDetails = []
    },
    CreateConversation: async () => main.Conversation.createFrom(FakeStore.createdConversationResult(store.createConversation())),
    DeleteAllConversations: async () => store.deleteAllConversations(),
    DeleteConversation: async id => store.deleteConversation(id),
    DeleteMessagesFrom: async (conversationID, index) => {
      if (index < 0) throw new Error(`invalid message index ${index}`)
      store.deleteMessagesFrom(conversationID, index)
    },
    ForgetMemory: async id => store.forgetMemory(id),
    GetAvailableProviders: async () => [
      { id: 'azure', name: 'Azure OpenAI (Login)' },
      { id: 'azure_apikey', name: 'Azure OpenAI (API Key)' },
      { id: 'openai', name: 'OpenAI' },
      { id: 'compat', name: 'Local / OpenAI-compatible (LM Studio, llama.cpp)' },
    ],
    GetConversations: async () =>
      store.listConversations().map(c => main.Conversation.createFrom({ ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt })),
    GetDefaultSystemPrompt: async () => DEFAULT_SYSTEM_PROMPT,
    GetGlobalRequestCount: async () => globalRequests,
    GetLLMDetails: async () => store.listLLMDetails().map(d => main.LLMDetailsEntry.createFrom(d)),
    GetLifetimeStats: async () => main.LifetimeStats.createFrom({ totalRequests: globalRequests, totalTokens: lifetimeTokens }),
    GetMemories: async () => store.listMemories().map(m => shared.MemoryRecord.createFrom(m)),
    GetMessages: async conversationID => store.getMessages(conversationID).map(m => main.Message.createFrom(m)),
    GetRequestCounts: async conversationID => {
      const conv = store.getConversation(conversationID)
      return main.RequestCounts.createFrom({ perChat: conv?.requestCount ?? 0, perChatTokens: conv?.requestTokens ?? 0, global: globalRequests })
    },
    GetSettings: async () => ({ ...store.settings }),
    IsAuthenticated: async () => llmConfigured() && store.settings.authenticated === 'true',
    OpenPath: async path => (path ? '' : 'empty path'),
    RegenerateMessage: async (conversationID, index) => {
      const invalid = validIndex(conversationID, index, 'assistant')
      if (invalid) return invalid
      store.deleteMessagesFrom(conversationID, index)
      if (!llmConfigured()) return 'Error: Please configure LLM provider settings first'
      return runChat(conversationID)
    },
    ResendMessage: async (conversationID, index) => {
      const invalid = validIndex(conversationID, index, 'user')
      if (invalid) return invalid
      store.deleteMessagesFrom(conversationID, index + 1)
      if (!llmConfigured()) return 'Error: Please configure LLM provider settings first'
      return runChat(conversationID)
    },
    SaveBase64File: async filename => main.SaveResult.createFrom({ path: `/Users/test/Downloads/${filename}`, cancelled: false }),
    SaveFile: async filename => main.SaveResult.createFrom({ path: `/Users/test/Downloads/${filename}`, cancelled: false }),
    SaveImage: async src => main.SaveResult.createFrom({ path: `/Users/test/Downloads/${src.split('/').pop() || 'image.png'}`, cancelled: false }),
    SaveMemory: async (key, value, category) => store.saveMemory(key, value, category),
    SaveSettings: async settings => {
      const next = settings.provider_type
      const prev = store.settings.provider_type
      if (next !== undefined && prev && prev !== next) store.settings.authenticated = 'false'
      for (const [key, value] of Object.entries(settings)) setSetting(key, value)
    },
    SearchMessages: async query => (query ? store.searchMessages(query) : {}),
    SelectFile: async () => fileSelections.shift() ?? '',
    SelectFolder: async () => folderSelections.shift() ?? '',
    SendMessage: (conversationID, content, attachmentsJSON) => sendMessage(conversationID, content, attachmentsJSON),
    SendMessageWithImages: (conversationID, content, _images, attachmentsJSON) => sendMessage(conversationID, content, attachmentsJSON),
    SetSetting: async (key, value) => setSetting(key, value),
    SignIn: async () => {
      if (!llmConfigured()) return 'Error: No LLM provider configured – save your settings first'
      store.settings.authenticated = 'true'
      return ''
    },
    SignOut: async () => {
      store.settings.authenticated = 'false'
    },
    TestConnection: async () => {
      if (!llmConfigured()) return 'Error: No LLM provider configured – save your settings first'
      const fingerprint = connectionSettingsFingerprint(store.settings)
      const outcome = seed.connectionTest ?? 'ok'
      const ok = outcome === 'ok'
      const message = ok ? 'Connection successful!' : outcome.error
      store.settings.authenticated = 'true'
      store.settings[CONNECTION_TEST_KEY] = JSON.stringify({ ok, message, at: Date.now(), fingerprint })
      return ok ? message : `Error: ${message}`
    },
  }

  const updater: FakeUpdater = {
    CheckForUpdate: async () =>
      main.UpdateInfo.createFrom({
        available: !!seed.update,
        currentVersion: seed.version ?? '1.6.0',
        latestVersion: seed.update?.latestVersion ?? seed.version ?? '1.6.0',
        releaseURL: 'https://example.com/releases',
        assetName: seed.update?.assetName ?? 'Pedro-macos-arm64.dmg',
        assetURL: seed.update?.assetURL ?? 'https://example.com/Pedro-macos-arm64.dmg',
      }),
    DownloadAndInstall: async () => {
      events.emit('update_progress', 0, 'downloading')
      await sleep(150)
      events.emit('update_progress', 100, 'installing')
      await sleep(150)
      events.emit('update_progress', 100, 'done')
    },
    GetCurrentVersion: async () => seed.version ?? '1.6.0',
    GetWindowsDefenderExclusion: async () => '',
  }

  /** Records every call and round-trips values through JSON, like the real IPC bridge. */
  function bridge<T extends Record<string, (...args: never[]) => unknown>>(impl: T): T {
    const out: Record<string, unknown> = {}
    for (const [method, fn] of Object.entries(impl)) {
      out[method] = async (...args: unknown[]) => {
        calls.push({ method, args: overTheWire(args) })
        if (seed.latencyMs) await sleep(seed.latencyMs)
        try {
          return overTheWire(await (fn as (...a: unknown[]) => unknown)(...overTheWire(args)))
        } catch (err) {
          // Go errors reach JavaScript as rejected promises carrying the message.
          throw err instanceof Error ? err.message : err
        }
      }
    }
    return out as T
  }

  const runtimeImpl: Record<string, unknown> = {
    EventsOnMultiple: (name: string, cb: (...data: unknown[]) => void, max: number) => events.on(name, cb, max),
    EventsOff: (name: string, ...more: string[]) => events.off(name, ...more),
    EventsOffAll: () => events.offAll(),
    EventsEmit: (name: string, ...data: unknown[]) => events.emit(name, ...data),
    BrowserOpenURL: (url: string) => {
      calls.push({ method: 'BrowserOpenURL', args: [url] })
    },
    OnFileDrop: (cb: (x: number, y: number, paths: string[]) => void) => {
      fileDrop = cb
    },
    OnFileDropOff: () => {
      fileDrop = null
    },
    Environment: async () => ({ buildType: 'dev', platform: 'darwin', arch: 'arm64' }),
  }
  // Window*, Log* and the other runtime calls are no-ops in a browser.
  const runtime = new Proxy(runtimeImpl, {
    get: (target, prop: string) => (prop in target ? target[prop] : () => undefined),
  })

  const control: FakeControl = {
    queueReply: reply => replies.push(reply),
    queueFileSelection: path => fileSelections.push(path),
    queueFolderSelection: path => folderSelections.push(path),
    dropFiles: paths => fileDrop?.(0, 0, paths),
    emit: (event, ...data) => events.emit(event, ...data),
    calls: method => overTheWire(method ? calls.filter(c => c.method === method) : calls),
    snapshot: () =>
      overTheWire({
        conversations: store.listConversations(),
        messages: store.messages,
        settings: store.settings,
        memories: store.listMemories(),
        llmDetails: store.llmDetails.length,
      }),
  }

  return { app: bridge(app), updater: bridge(updater), runtime, control, store, events }
}
