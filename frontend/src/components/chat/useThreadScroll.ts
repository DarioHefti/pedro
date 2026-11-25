import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'

/** Beyond this distance from the bottom the reader has left the tail, so auto-follow stops. */
const SCROLL_STICK_THRESHOLD_PX = 80

interface ThreadScrollInput {
  /** The thread's messages; a new array means the thread changed. */
  messages: readonly unknown[]
  /** A reply is streaming into the thread. */
  streaming: boolean
  /** Text and tool calls of the streaming reply. */
  streamContent: string
  streamToolCalls: readonly unknown[]
  /** The open conversation; switching jumps to its end. */
  conversationID: number | null | undefined
}

/**
 * Keeps the thread pinned to its newest content while the reader follows along, and stops
 * following once they scroll up to read — until they send or jump back to the latest message.
 */
export function useThreadScroll({ messages, streaming, streamContent, streamToolCalls, conversationID }: ThreadScrollInput) {
  const threadRef = useRef<HTMLDivElement>(null)
  /** While true, new content keeps the viewport pinned to the bottom. */
  const [stickToBottom, setStickToBottom] = useState(true)
  const [showJumpButton, setShowJumpButton] = useState(false)
  /** A smooth jump animates scrollTop; ignore the transient "away from bottom" meanwhile. */
  const suppressStickBreakRef = useRef(false)
  const prevConvIDRef = useRef<number | null>(null)
  /**
   * scrollTop while reading during a stream, synced every layout pass so it survives the stream
   * ending between chunks. Restoring scrollTop (not distance-from-bottom) keeps the view still
   * when the final bubble replaces the streaming one.
   */
  const lastScrollTopWhileReadingRef = useRef(0)
  const prevStreamingRef = useRef(streaming)

  const scrollToBottom = useCallback((behavior: ScrollBehavior) => {
    const el = threadRef.current
    if (!el) return
    const pane = el.parentElement
    if (pane && pane.scrollTop !== 0) pane.scrollTop = 0
    if (behavior === 'smooth') el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    else el.scrollTop = el.scrollHeight
  }, [])

  useEffect(() => {
    if (conversationID !== undefined && conversationID !== prevConvIDRef.current) {
      if (prevConvIDRef.current !== null) {
        setStickToBottom(true)
        scrollToBottom('auto')
      }
      prevConvIDRef.current = conversationID
    }
  }, [conversationID, scrollToBottom])

  /** Scrolling away from the tail stops following until Send or "jump to latest". */
  useEffect(() => {
    const el = threadRef.current
    if (!el) return
    const onScroll = () => {
      if (suppressStickBreakRef.current) return
      lastScrollTopWhileReadingRef.current = el.scrollTop
      const away = el.scrollHeight - el.scrollTop - el.clientHeight > SCROLL_STICK_THRESHOLD_PX
      setShowJumpButton(away)
      if (away) setStickToBottom(false)
    }
    onScroll()
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => el.removeEventListener('scroll', onScroll)
  }, [])

  useLayoutEffect(() => {
    const el = threadRef.current
    if (!el) return
    if (streaming && !stickToBottom) lastScrollTopWhileReadingRef.current = el.scrollTop

    const streamEnded = prevStreamingRef.current && !streaming
    prevStreamingRef.current = streaming

    if (stickToBottom && !suppressStickBreakRef.current) {
      scrollToBottom('auto')
    } else if (streamEnded) {
      const maxScroll = Math.max(0, el.scrollHeight - el.clientHeight)
      el.scrollTop = Math.min(Math.max(0, lastScrollTopWhileReadingRef.current), maxScroll)
    }
    setShowJumpButton(el.scrollHeight - el.scrollTop - el.clientHeight > SCROLL_STICK_THRESHOLD_PX)
  }, [messages, streamToolCalls, streaming, streamContent, stickToBottom, scrollToBottom])

  /** Images and diagrams resize the thread without a render; stay pinned while following. */
  useEffect(() => {
    if (!stickToBottom) return
    const el = threadRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(() => {
      if (!suppressStickBreakRef.current) scrollToBottom('auto')
    })
    for (const child of Array.from(el.children)) ro.observe(child)
    ro.observe(el)
    return () => ro.disconnect()
  }, [stickToBottom, scrollToBottom, messages.length])

  /** Follow the tail again (e.g. after sending). */
  const follow = useCallback(() => setStickToBottom(true), [])

  const jumpToLatest = useCallback(() => {
    setStickToBottom(true)
    suppressStickBreakRef.current = true
    scrollToBottom('smooth')
    window.setTimeout(() => {
      suppressStickBreakRef.current = false
      const el = threadRef.current
      if (!el) return
      setShowJumpButton(el.scrollHeight - el.scrollTop - el.clientHeight > SCROLL_STICK_THRESHOLD_PX)
    }, 400)
  }, [scrollToBottom])

  return { threadRef, showJumpButton, follow, jumpToLatest }
}
