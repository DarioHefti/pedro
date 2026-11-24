import { useCallback, useEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { conversationService, eventService, messageService, type Conversation } from '../services/wailsService'
import {
  appendUserMessage,
  buildLLMContent,
  buildThreadState,
  emptyThread,
  imageDataURLs,
  isErrorResponse,
  truncateThread,
  type Attachment,
  type ThreadState,
  type ToolCall,
} from './messageModel'

export type { Message } from '../services/wailsService'
export type { Attachment, FileAttachment, ThreadState, ToolCall } from './messageModel'

/** What an in-flight reply has streamed so far. */
export interface StreamBuffer {
  content: string
  toolCalls: ToolCall[]
}

interface UseMessagingOptions {
  /** Resolved current conversation ID (null when no conversation is open). */
  currentConvID: number | null
  /** Creates a new conversation and returns it. */
  createConversation: () => Promise<Conversation>
  /** Called after a new conversation is created so the parent can track the ID. */
  onConversationCreated: (id: number) => void
  /** Refresh the conversation list (e.g. to reflect the auto-generated title). */
  refreshConversations: () => Promise<void>
  /** Called after deleting messages emptied a conversation and it was deleted. */
  onConversationDeleted: (id: number) => void
  /** Surface an error message to the user. */
  showError: (message: string) => void
}

/**
 * Owns the open thread and every in-flight reply. Replies stream per conversation, so a reply
 * keeps arriving in the background while another chat is open; only one runs at a time.
 */
export function useMessaging({
  currentConvID,
  createConversation,
  onConversationCreated,
  refreshConversations,
  onConversationDeleted,
  showError,
}: UseMessagingOptions) {
  const [thread, setThread] = useState<ThreadState>(emptyThread)
  /** Per-conversation stream buffers while the LLM is responding. */
  const [streams, setStreams] = useState<Map<number, StreamBuffer>>(() => new Map())

  const buffersRef = useRef<Map<number, StreamBuffer>>(new Map())
  /** Conversations whose stream events are accepted (between openStream and closeStream). */
  const acceptingRef = useRef<Set<number>>(new Set())
  /** The conversation the backend's single AbortMessage targets. */
  const abortTargetRef = useRef<number | null>(null)
  const currentConvIDRef = useRef(currentConvID)
  /** Bumped by every write of the thread; async loads drop results that arrive after a newer write. */
  const threadSeqRef = useRef(0)

  useEffect(() => {
    currentConvIDRef.current = currentConvID
  }, [currentConvID])

  const isCurrent = (convID: number) => currentConvIDRef.current === convID

  /**
   * Shows buffered stream updates at most once per animation frame. Chunks can arrive far
   * faster than the thread re-renders; rendering each one queued up work that froze the UI
   * when a long reply ended. flushSync keeps the Wayland fix: the frame's update reaches the
   * DOM at once instead of being batched away.
   */
  const streamFrameRef = useRef(0)
  const scheduleStreamRender = useCallback(() => {
    if (streamFrameRef.current) return
    streamFrameRef.current = requestAnimationFrame(() => {
      streamFrameRef.current = 0
      flushSync(() => {
        setStreams(new Map(buffersRef.current))
      })
    })
  }, [])
  useEffect(() => () => cancelAnimationFrame(streamFrameRef.current), [])

  // Route stream events by conversation id so switching chats does not mix streams.
  useEffect(() => {
    const appendToStream = (convId: number, update: (buf: StreamBuffer) => StreamBuffer) => {
      if (Number.isNaN(convId) || !acceptingRef.current.has(convId)) return
      buffersRef.current.set(convId, update(buffersRef.current.get(convId) ?? { content: '', toolCalls: [] }))
      scheduleStreamRender()
    }

    const unsubscribers = [
      eventService.on('stream_chunk', (...args: unknown[]) => {
        if (args.length < 2) return
        const chunk = String(args[1])
        appendToStream(Number(args[0]), buf => ({ ...buf, content: buf.content + chunk }))
      }),
      eventService.on('tool_call', (...args: unknown[]) => {
        if (args.length < 3) return
        const call = { name: String(args[1]), argsJSON: String(args[2]) }
        appendToStream(Number(args[0]), buf => ({ ...buf, toolCalls: [...buf.toolCalls, call] }))
      }),
      eventService.on('conversation_updated', (...args: unknown[]) => {
        if (args.length < 1 || Number.isNaN(Number(args[0]))) return
        void refreshConversations()
      }),
    ]
    return () => unsubscribers.forEach(unsubscribe => unsubscribe())
  }, [refreshConversations, scheduleStreamRender])

  const showThread = useCallback((next: ThreadState) => {
    ++threadSeqRef.current
    setThread(next)
  }, [])

  /** Loads a conversation's stored thread. Clears synchronously first, so a frame never shows
   *  the previous conversation's messages under the new conversation's ID. */
  const load = useCallback(async (convID: number): Promise<void> => {
    const seq = ++threadSeqRef.current
    setThread(emptyThread())
    const stored = await conversationService.getMessages(convID)
    if (threadSeqRef.current !== seq) return
    setThread(buildThreadState(stored))
  }, [])

  /** Clears the thread (e.g. when "New Chat" is selected). */
  const clear = useCallback((): void => showThread(emptyThread()), [showThread])

  const openStream = useCallback((convID: number) => {
    abortTargetRef.current = convID
    acceptingRef.current.add(convID)
    buffersRef.current.set(convID, { content: '', toolCalls: [] })
    setStreams(new Map(buffersRef.current))
  }, [])

  const closeStream = useCallback((convID: number) => {
    acceptingRef.current.delete(convID)
    buffersRef.current.delete(convID)
    if (abortTargetRef.current === convID) abortTargetRef.current = null
    setStreams(new Map(buffersRef.current))
  }, [])

  /**
   * Runs one backend turn that streams a reply into `convID`, then shows the stored thread.
   * The stream row goes away in the same render that shows the stored reply, so the reply
   * neither flickers nor shows twice.
   */
  const runTurn = useCallback(
    async (convID: number, request: () => Promise<string>): Promise<void> => {
      openStream(convID)
      let closed = false
      try {
        const response = await request()
        const stored = await conversationService.getMessages(convID)
        if (isCurrent(convID)) {
          closeStream(convID)
          closed = true
          showThread(buildThreadState(stored))
        }
        await refreshConversations()
        if (isErrorResponse(response)) showError(response)
      } finally {
        if (!closed) closeStream(convID)
      }
    },
    [openStream, closeStream, showThread, refreshConversations, showError],
  )

  /** Sends a user message, creating the conversation first when needed. */
  const send = useCallback(
    async (content: string, attachments: Attachment[] = []): Promise<void> => {
      let convID = currentConvID
      if (!convID) {
        const conv = await createConversation()
        convID = conv.ID
        onConversationCreated(convID)
        // Keep the ref in sync now so the optimistic update below targets the new chat.
        currentConvIDRef.current = convID
        await refreshConversations()
      }

      if (isCurrent(convID)) {
        // flushSync renders the person's message at once (Wayland batches it away otherwise).
        flushSync(() => setThread(prev => appendUserMessage(prev, content, attachments)))
      }

      const id = convID
      const images = imageDataURLs(attachments)
      const llmContent = buildLLMContent(content, attachments)
      const attachmentsJSON = JSON.stringify(attachments)
      await runTurn(id, () =>
        images.length > 0
          ? messageService.sendWithImages(id, llmContent, images, attachmentsJSON)
          : messageService.send(id, llmContent, attachmentsJSON),
      )
    },
    [currentConvID, createConversation, onConversationCreated, refreshConversations, runTurn],
  )

  /** Replaces the reply at `index` (and drops anything after it) with a new one. */
  const regenerate = useCallback(
    async (index: number): Promise<void> => {
      const convID = currentConvID
      if (!convID || thread.messages[index]?.Role !== 'assistant') return
      setThread(prev => truncateThread(prev, index - 1))
      await runTurn(convID, () => messageService.regenerate(convID, index))
    },
    [currentConvID, thread.messages, runTurn],
  )

  /** Answers the question at `index` again, dropping everything after it. */
  const resendMessage = useCallback(
    async (index: number): Promise<void> => {
      const convID = currentConvID
      if (!convID || thread.messages[index]?.Role !== 'user') return
      setThread(prev => truncateThread(prev, index))
      await runTurn(convID, () => messageService.resend(convID, index))
    },
    [currentConvID, thread.messages, runTurn],
  )

  const stop = useCallback(() => {
    void messageService.abort()
    const convID = abortTargetRef.current
    if (convID != null) closeStream(convID)
  }, [closeStream])

  /** Deletes the question at `index` and everything after it; an emptied chat is deleted too. */
  const deleteMessage = useCallback(
    async (index: number): Promise<void> => {
      const convID = currentConvID
      if (!convID || thread.messages[index]?.Role !== 'user') return

      await messageService.deleteFrom(convID, index)
      const stored = await conversationService.getMessages(convID)
      if (isCurrent(convID)) showThread(buildThreadState(stored))
      if (stored.length === 0) {
        await conversationService.delete(convID)
        // The next message must start a new chat, not write into the deleted one.
        onConversationDeleted(convID)
      }
      await refreshConversations()
    },
    [currentConvID, thread.messages, showThread, refreshConversations, onConversationDeleted],
  )

  const stream = currentConvID != null ? (streams.get(currentConvID) ?? null) : null

  return {
    thread,
    /** The reply streaming into the open chat, or null. */
    stream,
    /** True while any conversation has a reply in flight (sending waits until it ends). */
    streamingBusy: streams.size > 0,
    load,
    clear,
    send,
    regenerate,
    resendMessage,
    stop,
    deleteMessage,
  }
}
