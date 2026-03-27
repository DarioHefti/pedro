import { useState, useRef, useCallback, useEffect } from 'react'
import {
  conversationService,
  messageService,
  eventService,
  uiConversationService,
  isWailsDevStub,
  type Conversation,
  type Message,
} from '../services/wailsService'

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
      setStreamingStreams(new Map(streamingBuffersRef.current))
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
      setStreamingStreams(new Map(streamingBuffersRef.current))
    }

    const unsubChunk = eventService.on('stream_chunk', onChunk)
    const unsubTool = eventService.on('tool_call', onTool)
    return () => {
      unsubChunk?.()
      unsubTool?.()
    }
  }, [])

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

  /** Load messages for a conversation and clear any in-session attachment maps.
   *  Clears state synchronously (before the async DB fetch) so React batches
   *  the clear with the caller's setCurrentConvID — no render frame shows
   *  the old conversation's messages under the new conversation's ID. */
  const load = useCallback(async (convID: number): Promise<void> => {
    const seq = ++msgSeqRef.current
    setMessages([])
    setMessageImages(new Map())
    setMessageFiles(new Map())
    const msgs = await conversationService.getMessages(convID)
    if (msgSeqRef.current !== seq) return
    setMessages(msgs ?? [])
  }, [])

  /** Clear all message state (e.g. when "New Chat" is selected). */
  const clear = useCallback((): void => {
    ++msgSeqRef.current
    setMessages([])
    setMessageImages(new Map())
    setMessageFiles(new Map())
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

      const prevUserRankToImages = extractUserRankMap(messages, messageImages)
      const prevUserRankToFiles = extractUserRankMap(messages, messageFiles)
      const newUserRank = messages.filter(m => m.Role === 'user').length

      const optimisticIdx = messages.length
      if (currentConvIDRef.current === convID) {
        setMessages(prev => [...prev, { Role: 'user', Content: content } as Message])
        if (imageDataURLs.length > 0) {
          setMessageImages(m => new Map(m).set(optimisticIdx, imageDataURLs))
        }
        if (fileAttachments.length > 0) {
          setMessageFiles(m => new Map(m).set(optimisticIdx, fileAttachments))
        }
      }

      prepareStreaming(convID)

      try {
        const response =
          imageDataURLs.length > 0
            ? await messageService.sendWithImages(convID, llmContent, imageDataURLs, selectedPersonaId)
            : await messageService.send(convID, llmContent, selectedPersonaId)

        const dbMsgs = (await conversationService.getMessages(convID)) ?? []

        const dbUserIndices = dbMsgs
          .map((m, i) => (m.Role === 'user' ? i : -1))
          .filter(i => i >= 0)

        const newImgMap = remapByRank(prevUserRankToImages, dbUserIndices)
        if (imageDataURLs.length > 0 && newUserRank < dbUserIndices.length) {
          newImgMap.set(dbUserIndices[newUserRank], imageDataURLs)
        }

        const newFilesMap = remapByRank(prevUserRankToFiles, dbUserIndices)
        if (fileAttachments.length > 0 && newUserRank < dbUserIndices.length) {
          newFilesMap.set(dbUserIndices[newUserRank], fileAttachments)
        }

        if (currentConvIDRef.current === convID) {
          ++msgSeqRef.current
          setMessageImages(newImgMap)
          setMessageFiles(newFilesMap)
          setMessages(dbMsgs)
        }

        await refreshConversations()

        if (response?.startsWith('Error:')) {
          showError(response)
        }
      } finally {
        cleanupStreaming(convID)
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [currentConvID, messages, messageImages, messageFiles, createConversation, onConversationCreated, refreshConversations, showError],
  )

  const regenerate = useCallback(
    async (index: number, selectedPersonaId: string): Promise<void> => {
      const convID = currentConvID
      if (!convID || uiConversationService.isVirtualConversation(convID)) return
      const msg = messages[index]
      if (msg?.Role !== 'assistant') return

      prepareStreaming(convID)

      try {
        const response = await messageService.regenerate(convID, selectedPersonaId)
        if (currentConvIDRef.current === convID) {
          if (response?.startsWith('Error:')) {
            showError(response)
          } else {
            ++msgSeqRef.current
            const dbMsgs = (await conversationService.getMessages(convID)) ?? []
            setMessages(dbMsgs)
          }
        } else if (!response?.startsWith('Error:')) {
          await refreshConversations()
        }
      } finally {
        cleanupStreaming(convID)
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

function buildLLMContent(content: string, attachments?: Attachment[]): string {
  const fileText = (attachments ?? [])
    .filter(a => a.type !== 'image')
    .map(a => {
      if (a.type === 'file-ref') {
        return (
          `[File: ${a.name}]\n` +
          `[Path: ${a.content}]\n` +
          `[Large file attached by path. Use the read_file tool with this path to read it in chunks.]`
        )
      }
      if (a.type === 'folder-ref') {
        return (
          `[Folder: ${a.name}]\n` +
          `[Path: ${a.content}]\n` +
          `[Folder attached by path. Use the show_file_tree tool with this path (depth as needed; paginate with offset if truncated), then read_file for specific files.]`
        )
      }
      return `[File: ${a.name}]\n${a.content}`
    })
    .join('\n\n')

  return fileText ? `${content}\n\n${fileText}` : content
}

function extractUserRankMap<T>(
  messages: Message[],
  indexMap: Map<number, T>,
): Map<number, T> {
  const result = new Map<number, T>()
  let rank = 0
  messages.forEach((m, i) => {
    if (m.Role === 'user') {
      const value = indexMap.get(i)
      if (value !== undefined) result.set(rank, value)
      rank++
    }
  })
  return result
}

function remapByRank<T>(
  rankMap: Map<number, T>,
  dbUserIndices: number[],
): Map<number, T> {
  const result = new Map<number, T>()
  rankMap.forEach((value, rank) => {
    if (rank < dbUserIndices.length) {
      result.set(dbUserIndices[rank], value)
    }
  })
  return result
}
