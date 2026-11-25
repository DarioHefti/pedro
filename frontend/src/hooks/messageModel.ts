import type { Message } from '../services/wailsService'
import { isAudioFilePath, isRasterImageFilePath } from '../utils/fileKinds'

/**
 * The chat thread's data model: what the composer sends, how persisted messages are decoded for
 * the thread, and the notes added to a message for the model. Pure functions only.
 */

export interface ToolCall {
  name: string
  argsJSON: string
  id?: string
}

export interface Attachment {
  type: 'text' | 'image' | 'file-ref' | 'folder-ref'
  /** For file-ref / folder-ref: full OS path. For text: file content. For image: data URL. */
  content: string
  name: string
}

export interface FileAttachment {
  name: string
  path: string
  /** "file" | "folder" — drives LLM hints (read_file vs show_file_tree). */
  type: string
}

/** A thread as the UI shows it: messages plus per-message extras keyed by message index. */
export interface ThreadState {
  messages: Message[]
  images: Map<number, string[]>
  files: Map<number, FileAttachment[]>
  toolCalls: Map<number, ToolCall[]>
}

export function emptyThread(): ThreadState {
  return { messages: [], images: new Map(), files: new Map(), toolCalls: new Map() }
}

export function imageDataURLs(attachments: Attachment[]): string[] {
  return attachments.filter(a => a.type === 'image').map(a => a.content)
}

export function fileAttachments(attachments: Attachment[]): FileAttachment[] {
  return attachments
    .filter(a => a.type === 'file-ref' || a.type === 'folder-ref')
    .map(a => ({ name: a.name, path: a.content, type: a.type === 'folder-ref' ? 'folder' : 'file' }))
}

function attachmentNote(a: Attachment): string {
  if (a.type === 'file-ref') {
    if (isRasterImageFilePath(a.content)) {
      return `[File: ${a.name}]\n[Path: ${a.content}]\n[Raster image attached by path; the image pixels are also included in this message for vision.]`
    }
    if (isAudioFilePath(a.content)) {
      return `[File: ${a.name}]\n[Path: ${a.content}]\n[Audio file attached by path. Use the transcribe_audio tool with this path to transcribe it.]`
    }
    return (
      `[File: ${a.name}]\n[Path: ${a.content}]\n` +
      `[Large file attached by path. For PDF or Office files use parse_document; for source code, logs, and plain text use read_file — always paginate with offset/limit.]`
    )
  }
  if (a.type === 'folder-ref') {
    return (
      `[Folder: ${a.name}]\n[Path: ${a.content}]\n` +
      `[Folder attached by path. Use the show_file_tree tool with this path (depth as needed; paginate with offset if truncated), then parse_document for PDF/Office files or read_file for text/code files.]`
    )
  }
  return `[File: ${a.name}]\n${a.content}`
}

/** The message text sent to the model: what the person typed plus a note for each non-image attachment. */
export function buildLLMContent(content: string, attachments: Attachment[] = []): string {
  const notes = attachments.filter(a => a.type !== 'image').map(attachmentNote).join('\n\n')
  return notes ? `${content}\n\n${notes}` : content
}

/** Removes the [File: …] / [Folder: …] notes buildLLMContent appended; they never show in the UI. */
export function stripFileAnnotations(content: string): string {
  return content.replace(/\n\n\[(File|Folder): [\s\S]*$/, '')
}

function parseJSONArray<T>(raw: string | undefined): T[] {
  if (!raw) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as T[]) : []
  } catch {
    return []
  }
}

/** Decodes persisted messages (Attachments / ToolCalls JSON) into the thread the UI renders. */
export function buildThreadState(messages: Message[]): ThreadState {
  const images = new Map<number, string[]>()
  const files = new Map<number, FileAttachment[]>()
  const toolCalls = new Map<number, ToolCall[]>()

  messages.forEach((msg, idx) => {
    const atts = parseJSONArray<Attachment>(msg.Attachments)
    const imgs = imageDataURLs(atts)
    if (imgs.length > 0) images.set(idx, imgs)
    const refs = fileAttachments(atts)
    if (refs.length > 0) files.set(idx, refs)

    const calls = parseJSONArray<ToolCall>(msg.ToolCalls).filter(
      tc => tc && typeof tc.name === 'string' && typeof tc.argsJSON === 'string',
    )
    if (calls.length > 0) toolCalls.set(idx, calls)
  })

  return { messages, images, files, toolCalls }
}

function mapEntries<T>(input: Map<number, T>, remap: (idx: number) => number | null): Map<number, T> {
  const out = new Map<number, T>()
  for (const [idx, value] of input) {
    const next = remap(idx)
    if (next !== null) out.set(next, value)
  }
  return out
}

/** Keeps messages up to and including `index`; a negative index empties the thread. */
export function truncateThread(thread: ThreadState, index: number): ThreadState {
  const keep = (idx: number) => (idx <= index ? idx : null)
  return {
    messages: thread.messages.slice(0, index + 1),
    images: mapEntries(thread.images, keep),
    files: mapEntries(thread.files, keep),
    toolCalls: mapEntries(thread.toolCalls, keep),
  }
}

/** Appends the person's message before the backend has stored it. */
export function appendUserMessage(thread: ThreadState, content: string, attachments: Attachment[]): ThreadState {
  const idx = thread.messages.length
  const imgs = imageDataURLs(attachments)
  const refs = fileAttachments(attachments)
  return {
    messages: [...thread.messages, { Role: 'user', Content: content } as Message],
    images: imgs.length > 0 ? new Map(thread.images).set(idx, imgs) : thread.images,
    files: refs.length > 0 ? new Map(thread.files).set(idx, refs) : thread.files,
    toolCalls: thread.toolCalls,
  }
}

export function isErrorResponse(response: string | undefined): boolean {
  return !!response?.startsWith('Error:')
}
