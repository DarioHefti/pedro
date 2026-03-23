import { useState, useRef, useCallback } from 'react'
import {
  conversationService,
  messageService,
  eventService,
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
  type: 'text' | 'image' | 'file-ref'
  /** For file-ref: full OS path. For text: file content. For image: data URL. */
  content: string
  name: string
}

export interface FileAttachment {
  name: string
  path: string
  type: string
}

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
  const [loading, setLoading] = useState(false)
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([])
  const [streamingContent, setStreamingContent] = useState('')
  // Keyed by the message array index of the associated user message.
  const [messageImages, setMessageImages] = useState<Map<number, string[]>>(new Map())
  const [messageFiles, setMessageFiles] = useState<Map<number, FileAttachment[]>>(new Map())

  // Refs to accumulate streamed data outside of React state batching.
  const toolCallsRef = useRef<ToolCall[]>([])
  const streamingContentRef = useRef<string>('')

  // ---------------------------------------------------------------------------
  // Public actions
  // ---------------------------------------------------------------------------

  /** Load messages for a conversation and clear any in-session attachment maps. */
  const load = useCallback(async (convID: number): Promise<void> => {
    const msgs = await conversationService.getMessages(convID)
    setMessages(msgs ?? [])
    setMessageImages(new Map())
    setMessageFiles(new Map())
  }, [])

  /** Clear all message state (e.g. when "New Chat" is selected). */
  const clear = useCallback((): void => {
    setMessages([])
    setMessageImages(new Map())
    setMessageFiles(new Map())
  }, [])

  /** Send a user message, optionally with file/image attachments. */
  const send = useCallback(
    async (content: string, attachments?: Attachment[]): Promise<void> => {
      let convID = currentConvID

      if (!convID) {
        const conv = await createConversation()
        convID = conv.ID
        onConversationCreated(convID)
        await refreshConversations()
      }

      const imageDataURLs = (attachments ?? [])
        .filter(a => a.type === 'image')
        .map(a => a.content)

      const fileAttachments: FileAttachment[] = (attachments ?? [])
        .filter(a => a.type === 'file-ref')
        .map(a => ({ name: a.name, path: a.content, type: 'file' }))

      const llmContent = buildLLMContent(content, attachments)

      // -----------------------------------------------------------------------
      // Snapshot current user-message ranks BEFORE the optimistic update.
      // Used later to remap attachment indices after the DB reload.
      // -----------------------------------------------------------------------
      const prevUserRankToImages = extractUserRankMap(messages, messageImages)
      const prevUserRankToFiles = extractUserRankMap(messages, messageFiles)
      const newUserRank = messages.filter(m => m.Role === 'user').length

      // Optimistic UI update
      const optimisticIdx = messages.length
      setMessages(prev => [...prev, { Role: 'user', Content: content } as Message])
      if (imageDataURLs.length > 0) {
        setMessageImages(m => new Map(m).set(optimisticIdx, imageDataURLs))
      }
      if (fileAttachments.length > 0) {
        setMessageFiles(m => new Map(m).set(optimisticIdx, fileAttachments))
      }

      prepareStreaming()

      try {
        const response =
          imageDataURLs.length > 0
            ? await messageService.sendWithImages(convID, llmContent, imageDataURLs)
            : await messageService.send(convID, llmContent)

        const dbMsgs = (await conversationService.getMessages(convID)) ?? []

        // Build DB user-index list for rank-based remapping.
        const dbUserIndices = dbMsgs
          .map((m, i) => (m.Role === 'user' ? i : -1))
          .filter(i => i >= 0)

        // Rebuild attachment maps using rank (stable across reloads).
        const newImgMap = remapByRank(prevUserRankToImages, dbUserIndices)
        if (imageDataURLs.length > 0 && newUserRank < dbUserIndices.length) {
          newImgMap.set(dbUserIndices[newUserRank], imageDataURLs)
        }

        const newFilesMap = remapByRank(prevUserRankToFiles, dbUserIndices)
        if (fileAttachments.length > 0 && newUserRank < dbUserIndices.length) {
          newFilesMap.set(dbUserIndices[newUserRank], fileAttachments)
        }

        setMessageImages(newImgMap)
        setMessageFiles(newFilesMap)
        setMessages(dbMsgs)

        await refreshConversations()

        if (response?.startsWith('Error:')) {
          showError(response)
        }
      } finally {
        cleanupStreaming()
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [currentConvID, messages, messageImages, messageFiles, createConversation, onConversationCreated, refreshConversations, showError],
  )

  /** Remove the last assistant message and ask the LLM to regenerate it. */
  const regenerate = useCallback(
    async (index: number): Promise<void> => {
      const msg = messages[index]
      if (msg?.Role !== 'assistant' || !currentConvID) return

      setMessages(prev => prev.slice(0, index))
      prepareStreaming()

      try {
        const response = await messageService.regenerate(currentConvID)
        if (response?.startsWith('Error:')) {
          showError(response)
        } else {
          setMessages(prev => [
            ...prev,
            { Role: 'assistant', Content: response } as Message,
          ])
        }
      } finally {
        cleanupStreaming()
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [messages, currentConvID, showError],
  )

  // ---------------------------------------------------------------------------
  // Streaming helpers (not exposed)
  // ---------------------------------------------------------------------------

  function prepareStreaming() {
    toolCallsRef.current = []
    streamingContentRef.current = ''
    setToolCalls([])
    setStreamingContent('')
    setLoading(true)

    // Defensively remove any lingering listeners before adding new ones to
    // prevent handler stacking if a previous call skipped its finally block.
    eventService.off('tool_call')
    eventService.off('stream_chunk')

    eventService.on('tool_call', (name: string, argsJSON: string) => {
      const tc: ToolCall = { name, argsJSON }
      toolCallsRef.current = [...toolCallsRef.current, tc]
      setToolCalls([...toolCallsRef.current])
    })

    eventService.on('stream_chunk', (chunk: string) => {
      streamingContentRef.current += chunk
      setStreamingContent(streamingContentRef.current)
    })
  }

  function cleanupStreaming() {
    eventService.off('tool_call')
    eventService.off('stream_chunk')
    streamingContentRef.current = ''
    setStreamingContent('')
    setLoading(false)
    setToolCalls([])
  }

  return {
    messages,
    loading,
    toolCalls,
    streamingContent,
    messageImages,
    messageFiles,
    load,
    clear,
    send,
    regenerate,
  }
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

/**
 * Build the content string sent to the LLM. File attachments (non-image) are
 * appended as structured text; image data URLs are handled via a separate
 * multimodal API call and are NOT embedded here.
 */
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
      return `[File: ${a.name}]\n${a.content}`
    })
    .join('\n\n')

  return fileText ? `${content}\n\n${fileText}` : content
}

/**
 * Extract a rank → attachment map from an index-keyed attachment map and the
 * current message list. "Rank" is the ordinal position among user messages
 * (0 = first user message, 1 = second, …).
 */
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

/**
 * Rebuild an index-keyed attachment map from a rank-keyed one and the new
 * array of DB user-message indices.
 */
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
