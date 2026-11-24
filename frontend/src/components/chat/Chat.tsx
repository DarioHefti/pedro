import { useCallback, useEffect, useLayoutEffect, useRef, useState, type DragEvent, type JSX } from 'react'
import { ArrowDown, Upload } from 'lucide-react'
import { useShortcut } from '@ui-sdk/react/shortcuts'
import { OnFileDrop, OnFileDropOff } from '../../../wailsjs/runtime/runtime'
import type { Attachment, FileAttachment, Message, ToolCall } from '../../hooks/useMessaging'
import { copyMessageContent } from '../../utils/clipboard'
import { toast } from '../../ui/toaster'
import { AssistantMessage, StreamingMessage, UserMessage } from './ChatMessages'
import { ChatHeader } from './ChatHeader'
import { Composer } from './Composer'
import { EmptyState, type SuggestionAttach } from './EmptyState'

interface ChatProps {
  title: string
  messages: Message[]
  toolCalls: ToolCall[]
  messageToolCalls: Map<number, ToolCall[]>
  /** This thread is showing an in-flight assistant stream. */
  loading: boolean
  /** Any conversation has an active stream (blocks sending elsewhere). */
  streamingBusy: boolean
  streamingContent: string
  messageImages: Map<number, string[]>
  messageFiles: Map<number, FileAttachment[]>
  onSend: (content: string, attachments: Attachment[] | undefined) => void
  onStop: () => void
  onRegenerate: (index: number) => void
  /** Deletes a user message and every message after it. */
  onDeleteMessage: (index: number) => void
  /** Truncates later turns and streams a new reply. */
  onResendMessage: (index: number) => void
  onSelectFile: () => Promise<string>
  onSelectFolder: () => Promise<string>
  welcomeMessage: string
  /** Increment to focus the composer. */
  focusTrigger?: number
  currentConvID?: number | null
  /** Skip refocusing the composer while a dialog owns focus. */
  composerFocusPaused?: boolean
  requestCount?: number
  requestTokens?: number
  onViewDetails?: () => void
  sidebarCollapsed: boolean
  onShowSidebar: () => void
  onNewChat: () => void
  onOpenPalette: () => void
}

/** buildLLMContent appends [File: …] / [Folder: …] notes for the model; they never show in the UI. */
function stripFileAnnotations(content: string): string {
  return content.replace(/\n\n\[(File|Folder): [\s\S]*$/, '')
}

function baseName(path: string): string {
  return path.split(/[\\/]/).pop() || path
}

function hasWailsRuntime(): boolean {
  return typeof window !== 'undefined' && !!(window as Window & { runtime?: { OnFileDrop?: unknown } }).runtime?.OnFileDrop
}

/** Beyond this distance from the bottom the reader has left the tail, so auto-follow stops. */
const SCROLL_STICK_THRESHOLD_PX = 80
const MAX_TEXT_ATTACHMENT = 50_000

export default function Chat({
  title,
  messages,
  toolCalls,
  messageToolCalls,
  loading,
  streamingBusy,
  streamingContent,
  messageImages,
  messageFiles,
  onSend,
  onStop,
  onRegenerate,
  onDeleteMessage,
  onResendMessage,
  onSelectFile,
  onSelectFolder,
  welcomeMessage,
  focusTrigger,
  currentConvID,
  composerFocusPaused = false,
  requestCount = 0,
  requestTokens = 0,
  onViewDetails,
  sidebarCollapsed,
  onShowSidebar,
  onNewChat,
  onOpenPalette,
}: ChatProps) {
  const [input, setInput] = useState('')
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [isDragging, setIsDragging] = useState(false)
  /** While true, new content keeps the viewport pinned to the bottom. */
  const [stickToBottom, setStickToBottom] = useState(true)
  const [showJumpButton, setShowJumpButton] = useState(false)

  const threadRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  /** A smooth jump animates scrollTop; ignore the transient "away from bottom" meanwhile. */
  const suppressStickBreakRef = useRef(false)
  const prevConvIDRef = useRef<number | null>(null)
  /**
   * scrollTop while reading during a stream, synced every layout pass so it survives the stream
   * ending between chunks. Restoring scrollTop (not distance-from-bottom) keeps the view still
   * when the final bubble replaces the streaming one.
   */
  const lastScrollTopWhileReadingRef = useRef(0)
  const prevLoadingRef = useRef(loading)

  const scrollToBottom = useCallback((behavior: ScrollBehavior) => {
    const el = threadRef.current
    if (!el) return
    const pane = el.parentElement
    if (pane && pane.scrollTop !== 0) pane.scrollTop = 0
    if (behavior === 'smooth') el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    else el.scrollTop = el.scrollHeight
  }, [])

  /**
   * WebView2 can ignore a single focus() right after window focus events, so retry for ~300ms and
   * stop as soon as focus lands.
   */
  const focusComposer = useCallback((retries = 6) => {
    const attempt = (remaining: number) => {
      const el = inputRef.current
      if (!el) return
      el.focus({ preventScroll: true })
      if (document.activeElement === el || remaining <= 0) return
      setTimeout(() => attempt(remaining - 1), 50)
    }
    requestAnimationFrame(() => attempt(retries))
  }, [])

  useEffect(() => {
    if (focusTrigger === undefined || focusTrigger === 0) return
    focusComposer()
  }, [focusTrigger, focusComposer])

  /**
   * In WebView2, focus() is ignored until the webview itself has OS focus. Redirect a click on
   * empty space to the composer, and refocus when the window comes back.
   */
  useEffect(() => {
    if (composerFocusPaused) return
    const INTERACTIVE = 'input, textarea, select, button, a, [contenteditable], [tabindex]'
    /** Clicks inside any open dialog (lightbox, diagram viewer, …) belong to that dialog. */
    const DIALOG = '[data-slot="dialog-content"], [data-slot="alert-dialog-content"], [role="dialog"]'

    const onPointerUp = (e: PointerEvent) => {
      const target = e.target as HTMLElement
      if (target.closest(INTERACTIVE) || target.closest(DIALOG)) return
      const selection = window.getSelection()
      if (selection && selection.toString().length > 0) return
      setTimeout(() => inputRef.current?.focus({ preventScroll: true }), 0)
    }
    const onWindowFocus = () => focusComposer(6)
    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') focusComposer(6)
    }

    document.addEventListener('pointerup', onPointerUp)
    window.addEventListener('focus', onWindowFocus)
    document.addEventListener('visibilitychange', onVisibilityChange)
    focusComposer(10)
    return () => {
      document.removeEventListener('pointerup', onPointerUp)
      window.removeEventListener('focus', onWindowFocus)
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  }, [composerFocusPaused, focusComposer])

  useEffect(() => {
    if (currentConvID !== undefined && currentConvID !== prevConvIDRef.current) {
      if (prevConvIDRef.current !== null) {
        setStickToBottom(true)
        scrollToBottom('auto')
      }
      prevConvIDRef.current = currentConvID
    }
  }, [currentConvID, scrollToBottom])

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
    if (loading && !stickToBottom) lastScrollTopWhileReadingRef.current = el.scrollTop

    const loadingEnded = prevLoadingRef.current && !loading
    prevLoadingRef.current = loading

    if (stickToBottom && !suppressStickBreakRef.current) {
      scrollToBottom('auto')
    } else if (loadingEnded) {
      const maxScroll = Math.max(0, el.scrollHeight - el.clientHeight)
      el.scrollTop = Math.min(Math.max(0, lastScrollTopWhileReadingRef.current), maxScroll)
    }
    setShowJumpButton(el.scrollHeight - el.scrollTop - el.clientHeight > SCROLL_STICK_THRESHOLD_PX)
  }, [messages, toolCalls, loading, streamingContent, stickToBottom, scrollToBottom])

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

  useShortcut(streamingBusy ? 'escape' : null, () => onStop(), { allowInInput: true, description: 'Stop generating' })

  const handleSend = () => {
    const content = input.trim()
    if (streamingBusy || (!content && attachments.length === 0)) return
    setStickToBottom(true)
    setInput('')
    setAttachments([])
    onSend(content, attachments)
  }

  const handleCopyMessage = useCallback(async (content: string) => {
    try {
      const kind = await copyMessageContent(content)
      if (kind === 'image') toast.success('Image copied to clipboard')
    } catch (err) {
      toast.error('Copy failed', { description: err instanceof Error ? err.message : String(err) })
      throw err
    }
  }, [])

  const processFile = useCallback((file: File) => {
    const reader = new FileReader()
    if (file.type.startsWith('image/')) {
      reader.onload = () => {
        setAttachments(prev => [...prev, { type: 'image', content: reader.result as string, name: file.name || 'Pasted image' }])
      }
      reader.readAsDataURL(file)
      return
    }
    reader.onload = () => {
      const text = reader.result as string
      const content =
        text.length > MAX_TEXT_ATTACHMENT
          ? text.slice(0, MAX_TEXT_ATTACHMENT) +
            `\n\n[File truncated — ${(text.length / 1024).toFixed(0)} KB total. Use Attach → File to reference the path for full access.]`
          : text
      setAttachments(prev => [...prev, { type: 'text', content, name: file.name }])
    }
    reader.readAsText(file)
  }, [])

  const handleSelectFile = useCallback(async () => {
    const path = await onSelectFile()
    if (!path) return
    setAttachments(prev => [...prev, { type: 'file-ref', content: path, name: baseName(path) }])
    focusComposer()
  }, [onSelectFile, focusComposer])

  const handleSelectFolder = useCallback(async () => {
    const path = await onSelectFolder()
    if (!path) return
    setAttachments(prev => [...prev, { type: 'folder-ref', content: path, name: baseName(path) }])
    focusComposer()
  }, [onSelectFolder, focusComposer])

  const handlePickSuggestion = useCallback(
    (prompt: string, attach?: SuggestionAttach) => {
      setInput(prompt)
      if (attach === 'file') void handleSelectFile()
      else if (attach === 'folder') void handleSelectFolder()
      else focusComposer()
    },
    [handleSelectFile, handleSelectFolder, focusComposer],
  )

  const handleDragOver = (e: DragEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }
  const handleDragLeave = (e: DragEvent) => {
    e.preventDefault()
    if (e.currentTarget.contains(e.relatedTarget as Node | null)) return
    setIsDragging(false)
  }
  const handleDrop = (e: DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    // In Wails, OnFileDrop delivers native paths; the File API is the browser fallback.
    if (hasWailsRuntime()) return
    for (const file of Array.from(e.dataTransfer?.files ?? [])) processFile(file)
  }

  useEffect(() => {
    if (!hasWailsRuntime()) return
    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      setIsDragging(false)
      setAttachments(prev => [...prev, ...paths.map(path => ({ type: 'file-ref' as const, content: path, name: baseName(path) }))])
    }, false)
    return () => {
      if (hasWailsRuntime()) OnFileDropOff()
    }
  }, [])

  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      for (const item of Array.from(e.clipboardData?.items ?? [])) {
        if (item.type.startsWith('image/')) {
          const file = item.getAsFile()
          if (file) processFile(file)
        }
      }
    }
    document.addEventListener('paste', onPaste)
    return () => document.removeEventListener('paste', onPaste)
  }, [processFile])

  const jumpToBottom = () => {
    setStickToBottom(true)
    suppressStickBreakRef.current = true
    scrollToBottom('smooth')
    window.setTimeout(() => {
      suppressStickBreakRef.current = false
      const el = threadRef.current
      if (!el) return
      setShowJumpButton(el.scrollHeight - el.scrollTop - el.clientHeight > SCROLL_STICK_THRESHOLD_PX)
    }, 400)
  }

  const rows: JSX.Element[] = []
  let prevRole: string | null = null
  messages.forEach((msg, i) => {
    if (msg.Role === 'tool') return
    const key = msg.ID ? `m${msg.ID}` : `i${i}`
    if (msg.Role === 'user') {
      rows.push(
        <UserMessage
          key={key}
          content={stripFileAnnotations(msg.Content || '')}
          images={messageImages.get(i)}
          files={messageFiles.get(i)}
          busy={streamingBusy}
          onResend={() => onResendMessage(i)}
          onDelete={() => onDeleteMessage(i)}
        />,
      )
    } else {
      const content = msg.Content || ''
      rows.push(
        <AssistantMessage
          key={key}
          content={content}
          toolCalls={messageToolCalls.get(i)}
          continuation={prevRole === 'assistant'}
          busy={streamingBusy}
          onCopy={() => handleCopyMessage(content)}
          onRegenerate={() => onRegenerate(i)}
        />,
      )
    }
    prevRole = msg.Role
  })

  const isEmpty = messages.length === 0 && !loading

  return (
    <main
      className="chat"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      data-empty={isEmpty ? '' : undefined}
    >
      <ChatHeader
        title={isEmpty ? 'New chat' : title}
        sidebarCollapsed={sidebarCollapsed}
        onShowSidebar={onShowSidebar}
        onNewChat={onNewChat}
        onOpenPalette={onOpenPalette}
        stats={currentConvID != null ? { requests: requestCount, tokens: requestTokens } : undefined}
        onViewDetails={onViewDetails}
      />

      <div className="thread" ref={threadRef}>
        {isEmpty ? (
          <EmptyState welcomeMessage={welcomeMessage} onPick={handlePickSuggestion} />
        ) : (
          <div className="thread-inner">
            {rows}
            {loading ? (
              <StreamingMessage content={streamingContent} toolCalls={toolCalls} continuation={prevRole === 'assistant'} />
            ) : null}
          </div>
        )}
      </div>

      <div className="composer-dock">
        {showJumpButton && messages.length > 0 ? (
          <button type="button" className="jump-latest" onClick={jumpToBottom}>
            <ArrowDown aria-hidden="true" />
            {loading ? 'New messages' : 'Jump to latest'}
          </button>
        ) : null}
        <Composer
          value={input}
          onValueChange={setInput}
          attachments={attachments}
          onRemoveAttachment={index => setAttachments(prev => prev.filter((_, i) => i !== index))}
          onSend={handleSend}
          onStop={onStop}
          busy={streamingBusy}
          onAttachFile={() => void handleSelectFile()}
          onAttachFolder={() => void handleSelectFolder()}
          inputRef={inputRef}
        />
      </div>

      {isDragging ? (
        <div className="drop-overlay" aria-hidden="true">
          <div className="drop-card">
            <span className="drop-icon">
              <Upload />
            </span>
            <strong>Drop to attach</strong>
            <span>Files are attached by path so Pedro can read them</span>
          </div>
        </div>
      ) : null}
    </main>
  )
}
