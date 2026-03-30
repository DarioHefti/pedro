import { useState, useRef, useCallback, useEffect } from 'react'
import { flushSync } from 'react-dom'
import {
  conversationService,
  messageService,
  eventService,
  uiConversationService,
  isWailsDevStub,
  type Conversation,
} from '../services/wailsService'
export type { Message } from '../services/wailsService'
import type { Message } from '../services/wailsService'

// ---------------------------------------------------------------------------
// Public types shared with components
// ---------------------------------------------------------------------------

export interface ToolCall {
  name: string
  argsJSON: string
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

function isDebugMockMermaid(): boolean {
  return typeof window !== 'undefined' && localStorage.getItem('debug_mock_mermaid') === 'true'
}

type StreamBuffer = { content: string; toolCalls: ToolCall[] }

// ---------------------------------------------------------------------------
// Hook options
// ---------------------------------------------------------------------------

interface UseMessagingOptions {
  /** Resolved current conversation ID (null when no conversation is open). */
  currentConvID: number | null
  /** Creates a new conversation and returns it. */
  createConversation: () => Promise<Conversation>
  /** Called after a new conversation is created so the parent can track the ID. */
  onConversationCreated: (id: number) => void
  /** Refresh the conversation list (e.g. to reflect the auto-generated title). */
  refreshConversations: () => Promise<void>
  /** Surface an error message to the user. */
  showError: (message: string) => void
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useMessaging({
  currentConvID,
  createConversation,
  onConversationCreated,
  refreshConversations,
  showError,
}: UseMessagingOptions) {
  const [messages, setMessages] = useState<Message[]>([])
  /** Per-conversation streaming buffers while the LLM is responding (supports background streams). */
  const [streamingStreams, setStreamingStreams] = useState<Map<number, StreamBuffer>>(() => new Map())
  const [messageImages, setMessageImages] = useState<Map<number, string[]>>(new Map())
  const [messageFiles, setMessageFiles] = useState<Map<number, FileAttachment[]>>(new Map())
  const [messageToolCalls, setMessageToolCalls] = useState<Map<number, ToolCall[]>>(new Map())

  const streamingBuffersRef = useRef<Map<number, StreamBuffer>>(new Map())
  /** Conv IDs that are allowed to accept stream events (set in prepareStreaming, cleared in cleanup). */
  const activeStreamConvIdsRef = useRef<Set<number>>(new Set())
  /** Which conversation the in-flight AbortMessage targets (single backend run). */
  const activeStreamingConvRef = useRef<number | null>(null)
  const currentConvIDRef = useRef(currentConvID)
  /** Monotonic counter — incremented every time a new load/clear/send-completion writes messages.
   *  Async DB fetches compare their captured value to skip stale responses. */
  const msgSeqRef = useRef(0)

  useEffect(() => {
    currentConvIDRef.current = currentConvID
  }, [currentConvID])

  // Route stream events by conversation id so switching chats does not mix streams.
  useEffect(() => {
    if (isWailsDevStub) return

    const onChunk = (...args: unknown[]) => {
      if (args.length < 2) return
      const convId = Number(args[0])
      const chunk = String(args[1])
      if (Number.isNaN(convId) || !activeStreamConvIdsRef.current.has(convId)) return
      const buf = streamingBuffersRef.current.get(convId) ?? { content: '', toolCalls: [] }
      buf.content += chunk
      streamingBuffersRef.current.set(convId, buf)
      // Use flushSync to force immediate DOM update - fixes Wayland event batching issues
      flushSync(() => {
        setStreamingStreams(new Map(streamingBuffersRef.current))
      })
    }

    const onTool = (...args: unknown[]) => {
      if (args.length < 3) return
      const convId = Number(args[0])
      const name = String(args[1])
      const argsJSON = String(args[2])
      if (Number.isNaN(convId) || !activeStreamConvIdsRef.current.has(convId)) return
      const buf = streamingBuffersRef.current.get(convId) ?? { content: '', toolCalls: [] }
      buf.toolCalls = [...buf.toolCalls, { name, argsJSON }]
      streamingBuffersRef.current.set(convId, buf)
      // Use flushSync to force immediate DOM update - fixes Wayland event batching issues
      flushSync(() => {
        setStreamingStreams(new Map(streamingBuffersRef.current))
      })
    }

    const onConversationUpdated = (...args: unknown[]) => {
      if (args.length < 1) return
      const convId = Number(args[0])
      if (Number.isNaN(convId)) return
      void refreshConversations()
    }

    const unsubChunk = eventService.on('stream_chunk', onChunk)
    const unsubTool = eventService.on('tool_call', onTool)
    const unsubConversationUpdated = eventService.on(
      'conversation_updated',
      onConversationUpdated,
    )
    return () => {
      unsubChunk?.()
      unsubTool?.()
      unsubConversationUpdated?.()
    }
  }, [refreshConversations])

  const streamingForCurrent =
    currentConvID != null && streamingStreams.has(currentConvID)
      ? streamingStreams.get(currentConvID)!
      : { content: '', toolCalls: [] }

  /** True when the visible thread is showing an in-flight assistant stream. */
  const loading = currentConvID != null && streamingStreams.has(currentConvID)
  /** True when any conversation has an active stream (blocks new sends until done or stop). */
  const streamingBusy = streamingStreams.size > 0

  const toolCalls = streamingForCurrent.toolCalls
  const streamingContent = streamingForCurrent.content

  /** Load messages for a conversation and rebuild attachment maps from DB data.
   *  Clears state synchronously (before the async DB fetch) so React batches
   *  the clear with the caller's setCurrentConvID — no render frame shows
   *  the old conversation's messages under the new conversation's ID. */
  const load = useCallback(async (convID: number): Promise<void> => {
    const seq = ++msgSeqRef.current
    setMessages([])
    setMessageImages(new Map())
    setMessageFiles(new Map())
    setMessageToolCalls(new Map())

    if (isDebugMockMermaid() && msgSeqRef.current === 1) {
      const mockMessages = [
        { Role: 'user', Content: 'Show me a mermaid diagram' } as Message,
        { Role: 'assistant', Content: `Here is a diagram showing the flow:

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

This shows the key derivation and encryption flow.` } as Message,
      ]
      setMessages(mockMessages)
      return
    }

    const msgs = await conversationService.getMessages(convID)
    if (msgSeqRef.current !== seq) return
    const list = msgs ?? []
    setMessages(list)
    const { images, files } = buildAttachmentMaps(list)
    const toolCallsByMessageIndex = buildToolCallMaps(list)
    setMessageImages(images)
    setMessageFiles(files)
    setMessageToolCalls(toolCallsByMessageIndex)
  }, [])

  /** Clear all message state (e.g. when "New Chat" is selected). */
  const clear = useCallback((): void => {
    ++msgSeqRef.current
    setMessages([])
    setMessageImages(new Map())
    setMessageFiles(new Map())
    setMessageToolCalls(new Map())
  }, [])

  function prepareStreaming(convId: number) {
    activeStreamingConvRef.current = convId
    activeStreamConvIdsRef.current.add(convId)
    streamingBuffersRef.current.set(convId, { content: '', toolCalls: [] })
    setStreamingStreams(new Map(streamingBuffersRef.current))
  }

  function cleanupStreaming(convId: number) {
    activeStreamConvIdsRef.current.delete(convId)
    streamingBuffersRef.current.delete(convId)
    if (activeStreamingConvRef.current === convId) {
      activeStreamingConvRef.current = null
    }
    setStreamingStreams(new Map(streamingBuffersRef.current))
  }

  /**
   * Send a user message. selectedPersonaId is the chosen persona row id (backend loads prompt from DB).
   */
  const send = useCallback(
    async (
      content: string,
      attachments: Attachment[] | undefined,
      selectedPersonaId: string,
    ): Promise<void> => {
      let convID = currentConvID

      if (!convID || uiConversationService.isVirtualConversation(convID)) {
        const conv = await createConversation()
        convID = conv.ID
        onConversationCreated(convID)
        // Keep local ref in sync immediately so optimistic updates can run in this tick.
        currentConvIDRef.current = convID
        await refreshConversations()
      }

      const imageDataURLs = (attachments ?? [])
        .filter(a => a.type === 'image')
        .map(a => a.content)

      const fileAttachments: FileAttachment[] = (attachments ?? [])
        .filter(a => a.type === 'file-ref' || a.type === 'folder-ref')
        .map(a => ({
          name: a.name,
          path: a.content,
          type: a.type === 'folder-ref' ? 'folder' : 'file',
        }))

      const llmContent = buildLLMContent(content, attachments)
      const attachmentsJSON = JSON.stringify(attachments ?? [])

      const optimisticIdx = messages.length
      if (currentConvIDRef.current === convID) {
        // Use flushSync to ensure user message renders immediately on Wayland
        flushSync(() => {
          setMessages(prev => [...prev, { Role: 'user', Content: content } as Message])
          if (imageDataURLs.length > 0) {
            setMessageImages(m => new Map(m).set(optimisticIdx, imageDataURLs))
          }
          if (fileAttachments.length > 0) {
            setMessageFiles(m => new Map(m).set(optimisticIdx, fileAttachments))
          }
        })
      }

      prepareStreaming(convID)
      let streamClosed = false

      try {
        const response =
          imageDataURLs.length > 0
            ? await messageService.sendWithImages(convID, llmContent, imageDataURLs, selectedPersonaId, attachmentsJSON)
            : await messageService.send(convID, llmContent, selectedPersonaId, attachmentsJSON)

        const dbMsgs = (await conversationService.getMessages(convID)) ?? []

        if (currentConvIDRef.current === convID) {
          // Close the stream before committing final DB messages to avoid a one-frame duplicate
          // assistant bubble (stream row + finalized assistant row).
          cleanupStreaming(convID)
          streamClosed = true
          ++msgSeqRef.current
          const { images, files } = buildAttachmentMaps(dbMsgs)
          const toolCallsByMessageIndex = buildToolCallMaps(dbMsgs)
          setMessageImages(images)
          setMessageFiles(files)
          setMessageToolCalls(toolCallsByMessageIndex)
          setMessages(dbMsgs)
        }

        await refreshConversations()

        if (response?.startsWith('Error:')) {
          showError(response)
        }
      } finally {
        if (!streamClosed) cleanupStreaming(convID)
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [currentConvID, messages, createConversation, onConversationCreated, refreshConversations, showError],
  )

  const regenerate = useCallback(
    async (index: number, selectedPersonaId: string): Promise<void> => {
      const convID = currentConvID
      if (!convID || uiConversationService.isVirtualConversation(convID)) return
      const msg = messages[index]
      if (msg?.Role !== 'assistant') return

      if (currentConvIDRef.current === convID) {
        setMessages(prev => prev.filter((_, i) => i !== index))
        setMessageImages(prev => reindexMapWithoutIndex(prev, index))
        setMessageFiles(prev => reindexMapWithoutIndex(prev, index))
        setMessageToolCalls(prev => reindexMapWithoutIndex(prev, index))
      }

      prepareStreaming(convID)
      let streamClosed = false

      try {
        const response = await messageService.regenerate(convID, index, selectedPersonaId)
        if (currentConvIDRef.current === convID) {
          if (response?.startsWith('Error:')) {
            showError(response)
            ++msgSeqRef.current
            const dbMsgs = (await conversationService.getMessages(convID)) ?? []
            const { images, files } = buildAttachmentMaps(dbMsgs)
            const toolCallsByMessageIndex = buildToolCallMaps(dbMsgs)
            setMessageImages(images)
            setMessageFiles(files)
            setMessageToolCalls(toolCallsByMessageIndex)
            setMessages(dbMsgs)
          } else {
            // Same handoff fix as send(): remove stream row first, then render final DB row.
            cleanupStreaming(convID)
            streamClosed = true
            ++msgSeqRef.current
            const dbMsgs = (await conversationService.getMessages(convID)) ?? []
            const { images, files } = buildAttachmentMaps(dbMsgs)
            const toolCallsByMessageIndex = buildToolCallMaps(dbMsgs)
            setMessageImages(images)
            setMessageFiles(files)
            setMessageToolCalls(toolCallsByMessageIndex)
            setMessages(dbMsgs)
          }
        } else if (!response?.startsWith('Error:')) {
          await refreshConversations()
        }
      } finally {
        if (!streamClosed) cleanupStreaming(convID)
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [messages, currentConvID, showError, refreshConversations],
  )

  const stop = useCallback(() => {
    messageService.abort()
    const convId = activeStreamingConvRef.current
    if (convId != null) {
      cleanupStreaming(convId)
    }
  }, [])

  return {
    messages,
    loading,
    streamingBusy,
    toolCalls,
    messageToolCalls,
    streamingContent,
    messageImages,
    messageFiles,
    load,
    clear,
    send,
    regenerate,
    stop,
  }
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

/** Matches raster image paths for file-ref attachments (aligned with backend image_file_refs.go). */
function isRasterImageFilePath(path: string): boolean {
  return /\.(png|jpe?g|gif|webp|bmp|tiff?)$/i.test(path.trim())
}

function buildLLMContent(content: string, attachments?: Attachment[]): string {
  const fileText = (attachments ?? [])
    .filter(a => a.type !== 'image')
    .map(a => {
      if (a.type === 'file-ref') {
        if (isRasterImageFilePath(a.content)) {
          return (
            `[File: ${a.name}]\n` +
            `[Path: ${a.content}]\n` +
            `[Raster image attached by path; the image pixels are also included in this message for vision.]`
          )
        }
        return (
          `[File: ${a.name}]\n` +
          `[Path: ${a.content}]\n` +
          `[Large file attached by path. For PDF or Office files use parse_document; for source code, logs, and plain text use read_file — always paginate with offset/limit.]`
        )
      }
      if (a.type === 'folder-ref') {
        return (
          `[Folder: ${a.name}]\n` +
          `[Path: ${a.content}]\n` +
          `[Folder attached by path. Use the show_file_tree tool with this path (depth as needed; paginate with offset if truncated), then parse_document for PDF/Office files or read_file for text/code files.]`
        )
      }
      return `[File: ${a.name}]\n${a.content}`
    })
    .join('\n\n')

  return fileText ? `${content}\n\n${fileText}` : content
}

/** Parse each message's persisted Attachments JSON into index-keyed maps
 *  that Chat.tsx already consumes (messageImages, messageFiles). */
function buildAttachmentMaps(msgs: Message[]): {
  images: Map<number, string[]>
  files: Map<number, FileAttachment[]>
} {
  const images = new Map<number, string[]>()
  const files = new Map<number, FileAttachment[]>()

  msgs.forEach((msg, idx) => {
    if (!msg.Attachments) return
    try {
      const atts = JSON.parse(msg.Attachments) as Attachment[]
      const imgUrls = atts.filter(a => a.type === 'image').map(a => a.content)
      if (imgUrls.length > 0) images.set(idx, imgUrls)

      const fileAtts: FileAttachment[] = atts
        .filter(a => a.type === 'file-ref' || a.type === 'folder-ref')
        .map(a => ({
          name: a.name,
          path: a.content,
          type: a.type === 'folder-ref' ? 'folder' : 'file',
        }))
      if (fileAtts.length > 0) files.set(idx, fileAtts)
    } catch {
      /* ignore malformed JSON */
    }
  })

  return { images, files }
}

/** Parse each message's persisted ToolCalls JSON into an index-keyed map. */
function buildToolCallMaps(msgs: Message[]): Map<number, ToolCall[]> {
  const out = new Map<number, ToolCall[]>()

  msgs.forEach((msg, idx) => {
    const persisted = msg.ToolCalls
    if (!persisted) return
    try {
      const parsed = JSON.parse(persisted) as ToolCall[]
      const clean = parsed.filter(
        tc => tc && typeof tc.name === 'string' && typeof tc.argsJSON === 'string',
      )
      if (clean.length > 0) out.set(idx, clean)
    } catch {
      /* ignore malformed JSON */
    }
  })

  return out
}

function reindexMapWithoutIndex<T>(input: Map<number, T>, removedIndex: number): Map<number, T> {
  const out = new Map<number, T>()
  for (const [idx, value] of input.entries()) {
    if (idx === removedIndex) continue
    out.set(idx > removedIndex ? idx - 1 : idx, value)
  }
  return out
}
