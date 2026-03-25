import { useState, useRef, useEffect, useLayoutEffect, useCallback } from 'react'
import { OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime'
import attachmentIcon from './assets/attachment.svg'
import pedroAvatar from './assets/images/pedro.svg'
import type { Message, FileAttachment, ToolCall, Attachment } from './hooks/useMessaging'
import MessageRenderer from './MessageRenderer'
import AssistantMessageActions from './AssistantMessageActions'

interface ChatProps {
  messages: Message[]
  toolCalls: ToolCall[]
  loading: boolean
  streamingContent: string
  messageImages: Map<number, string[]>
  messageFiles: Map<number, FileAttachment[]>
  onSend: (content: string, attachments?: Attachment[]) => void
  onStop: () => void
  onRegenerate: (index: number) => void
  /** Opens the native OS file picker; resolves to the selected path or "". */
  onSelectFile: () => Promise<string>
  /** Opens the native folder picker; resolves to the selected path or "". */
  onSelectFolder: () => Promise<string>
  welcomeMessage: string
}

/**
 * Strip [File: ...] / [Folder: ...] / [Path: ...] annotations that buildLLMContent appends to
 * user messages before sending to the LLM. These are stored verbatim in the DB
 * but should never be shown in the chat UI.
 */
function stripFileAnnotations(content: string): string {
  return content.replace(/\n\n\[(File|Folder): [\s\S]*$/, '')
}

function toolCallArgDisplay(tc: ToolCall): string {
  let argsDisplay = ''
  try {
    const args = JSON.parse(tc.argsJSON) as Record<string, unknown>
    if (tc.name === 'websearch' && typeof args.query === 'string') {
      argsDisplay = `"${args.query}"`
    } else if (tc.name === 'webfetch' && typeof args.url === 'string') {
      argsDisplay = args.url
    } else if (typeof args.url === 'string') {
      argsDisplay = args.url
    } else if (typeof args.query === 'string') {
      argsDisplay = args.query
    } else if (typeof args.path === 'string') {
      argsDisplay = args.path
    } else if (typeof args.pattern === 'string') {
      argsDisplay = args.pattern
    } else if (typeof args.include === 'string') {
      argsDisplay = args.include
    }
  } catch {
    /* ignore */
  }
  return argsDisplay
}

/** Pixels from bottom beyond which we treat the user as having left the tail (stop auto-follow). */
const SCROLL_STICK_THRESHOLD_PX = 80

export default function Chat({
  messages,
  toolCalls,
  loading,
  streamingContent,
  messageImages,
  messageFiles,
  onSend,
  onStop,
  onRegenerate,
  onSelectFile,
  onSelectFolder,
  welcomeMessage,
}: ChatProps) {
  const [input, setInput] = useState('')
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [isDragging, setIsDragging] = useState(false)
  /** When true, new assistant content / stream keeps the viewport pinned to the bottom. */
  const [stickToBottom, setStickToBottom] = useState(true)
  const [showJumpButton, setShowJumpButton] = useState(false)
  const [attachMenuOpen, setAttachMenuOpen] = useState(false)

  const bottomRef = useRef<HTMLDivElement>(null)
  const messagesRef = useRef<HTMLDivElement>(null)
  const attachWrapRef = useRef<HTMLDivElement>(null)
  /** Smooth jump-to-bottom animates scrollTop; ignore transient “away from bottom” during that window. */
  const suppressStickBreakRef = useRef(false)

  const scrollMessagesToBottom = useCallback((behavior: ScrollBehavior) => {
    bottomRef.current?.scrollIntoView({ block: 'end', behavior })
  }, [])

  /** User scrolled away from the tail → stop following. Following only resumes on Send or “jump to bottom”. */
  useEffect(() => {
    const el = messagesRef.current
    if (!el) return

    const onScroll = () => {
      if (suppressStickBreakRef.current) return
      const distance = el.scrollHeight - el.scrollTop - el.clientHeight
      const away = distance > SCROLL_STICK_THRESHOLD_PX
      setShowJumpButton(away)
      if (away) {
        setStickToBottom(false)
      }
    }

    onScroll()
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => el.removeEventListener('scroll', onScroll)
  }, [])

  useLayoutEffect(() => {
    const el = messagesRef.current
    if (!el) return
    if (stickToBottom) {
      scrollMessagesToBottom('auto')
    }
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight
    setShowJumpButton(distance > SCROLL_STICK_THRESHOLD_PX)
  }, [messages, toolCalls, loading, streamingContent, stickToBottom, scrollMessagesToBottom])

  /** Stream / images can resize the thread without changing React deps; keep pinned when following. */
  useEffect(() => {
    if (!stickToBottom) return
    const el = messagesRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(() => {
      scrollMessagesToBottom('auto')
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [stickToBottom, scrollMessagesToBottom])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleSend = () => {
    const content = input.trim()
    if (!content && attachments.length === 0) return
    setStickToBottom(true)
    setInput('')
    setAttachments([])
    onSend(content, attachments)
  }

  const processFile = useCallback((file: File) => {
    if (file.type.startsWith('image/')) {
      const reader = new FileReader()
      reader.onload = () => {
        setAttachments(prev => [
          ...prev,
          { type: 'image', content: reader.result as string, name: file.name },
        ])
      }
      reader.readAsDataURL(file)
    } else {
      const reader = new FileReader()
      reader.onload = () => {
        const text = reader.result as string
        const MAX = 50_000
        const content =
          text.length > MAX
            ? text.slice(0, MAX) +
              `\n\n[File truncated — ${(text.length / 1024).toFixed(0)} KB total. Use Attach → File to reference the path for full access.]`
            : text
        setAttachments(prev => [...prev, { type: 'text', content, name: file.name }])
      }
      reader.readAsText(file)
    }
  }, [])

  const handleSelectFile = useCallback(async () => {
    const path = await onSelectFile()
    if (!path) return
    const name = path.split(/[\\/]/).pop() || path
    setAttachments(prev => [...prev, { type: 'file-ref', content: path, name }])
  }, [onSelectFile])

  const handleSelectFolder = useCallback(async () => {
    const path = await onSelectFolder()
    if (!path) return
    const name = path.split(/[\\/]/).pop() || path
    setAttachments(prev => [...prev, { type: 'folder-ref', content: path, name }])
  }, [onSelectFolder])

  useEffect(() => {
    if (!attachMenuOpen) return
    const onDocMouseDown = (ev: MouseEvent) => {
      const el = attachWrapRef.current
      if (el && !el.contains(ev.target as Node)) setAttachMenuOpen(false)
    }
    const onKeyDown = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') setAttachMenuOpen(false)
    }
    document.addEventListener('mousedown', onDocMouseDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onDocMouseDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [attachMenuOpen])

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }
  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
  }
  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    // Wails: native paths come from OnFileDrop. Browser: use File API.
    const rt = typeof window !== 'undefined' ? (window as Window & { runtime?: { OnFileDrop?: unknown } }).runtime : undefined
    if (rt?.OnFileDrop) return
    const files = Array.from(e.dataTransfer?.files ?? [])
    for (const file of files) processFile(file)
  }

  // Use Wails' OnFileDrop to get native file paths for drag & drop
  // useDropTarget=false means accept drops anywhere without requiring CSS properties
  useEffect(() => {
    const rt = typeof window !== 'undefined' ? (window as Window & { runtime?: { OnFileDrop?: unknown } }).runtime : undefined
    if (!rt?.OnFileDrop) return
    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      setIsDragging(false)
      for (const path of paths) {
        const name = path.split(/[\\/]/).pop() || path
        setAttachments(prev => [...prev, { type: 'file-ref', content: path, name }])
      }
    }, false)
    return () => {
      const rtOff = typeof window !== 'undefined' ? (window as Window & { runtime?: { OnFileDropOff?: unknown } }).runtime : undefined
      if (rtOff?.OnFileDropOff) OnFileDropOff()
    }
  }, [])

  const handlePaste = useCallback(
    (e: ClipboardEvent) => {
      const items = e.clipboardData?.items
      if (!items) return
      for (const item of Array.from(items)) {
        if (item.type.startsWith('image/')) {
          const file = item.getAsFile()
          if (file) processFile(file)
        }
      }
    },
    [processFile],
  )

  useEffect(() => {
    document.addEventListener('paste', handlePaste)
    return () => document.removeEventListener('paste', handlePaste)
  }, [handlePaste])

  const removeAttachment = (index: number) =>
    setAttachments(prev => prev.filter((_, i) => i !== index))

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

  const jumpToBottom = () => {
    setStickToBottom(true)
    suppressStickBreakRef.current = true
    scrollMessagesToBottom('smooth')
    window.setTimeout(() => {
      suppressStickBreakRef.current = false
      const el = messagesRef.current
      if (!el) return
      const distance = el.scrollHeight - el.scrollTop - el.clientHeight
      setShowJumpButton(distance > SCROLL_STICK_THRESHOLD_PX)
    }, 400)
  }

  const lastUserMessageIndex = messages.reduce(
    (acc, m, idx) => (m.Role === 'user' ? idx : acc),
    -1,
  )
  /** Tool rows belong after the latest user turn and before its assistant reply (incl. streaming). */
  const splitMessagesForToolCalls =
    toolCalls.length > 0 && lastUserMessageIndex >= 0
  const messageHeadCount = splitMessagesForToolCalls
    ? lastUserMessageIndex + 1
    : messages.length

  function renderMessageRow(i: number) {
    const msg = messages[i]
    const imgs = messageImages.get(i)
    const files = messageFiles.get(i)

    const bubble = (
      <div
        className={
          msg.Role === 'assistant'
            ? 'message assistant assistant-reply-bubble'
            : `message ${msg.Role}`
        }
      >
        {imgs && imgs.length > 0 && (
          <div className="message-image-previews">
            {imgs.map((src, j) => (
              <img
                key={j}
                src={src}
                alt={`Attached image ${j + 1}`}
                className="message-image-thumb"
              />
            ))}
          </div>
        )}
        {files && files.length > 0 && (
          <div className="message-file-previews">
            {files.map((file, j) => (
              <div key={j} className="message-file-chip" title={file.path}>
                <span className="message-file-icon">
                  {file.type === 'folder' ? '📁' : '📄'}
                </span>
                <span className="message-file-name">{file.name}</span>
              </div>
            ))}
          </div>
        )}
        <MessageRenderer
          content={msg.Role === 'user' ? stripFileAnnotations(msg.Content || '') : (msg.Content || '')}
          role={msg.Role}
        />
      </div>
    )

    if (msg.Role === 'assistant') {
      return (
        <div key={i} className="message-wrapper assistant">
          <div className="assistant-bubble-shell">
            {bubble}
            <AssistantMessageActions
              visible
              onCopy={() => void navigator.clipboard.writeText(msg.Content || '')}
              onRegenerate={() => void onRegenerate(i)}
              copyDisabled={!(msg.Content || '').trim()}
              regenerateDisabled={loading}
            />
          </div>
        </div>
      )
    }

    return (
      <div key={i} className={`message-wrapper ${msg.Role}`}>
        {bubble}
      </div>
    )
  }

  function renderToolCallRows() {
    return toolCalls.map((tc, i) => {
      const argsDisplay = toolCallArgDisplay(tc)
      return (
        <div key={`tc-${i}`} className="message tool">
          <div className="tool-call-card">
            <span className="tool-call-icon">🔧</span>
            <span className="tool-call-label">{tc.name}</span>
            {argsDisplay ? <span className="tool-call-arg">{argsDisplay}</span> : null}
          </div>
        </div>
      )
    })
  }

  return (
    <div
      className="main"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {isDragging && (
        <div className="drop-overlay">
          <div className="drop-message">Drop files here to attach by path</div>
        </div>
      )}

      {showJumpButton && messages.length > 0 && (
        <div className="jump-buttons">
          <button onClick={jumpToBottom} title="Jump to bottom">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </button>
        </div>
      )}

      <div className="messages" ref={messagesRef}>
        {messages.length === 0 && !loading && (
          <div className="empty-state">
            <div className="empty-state-inner">
              <p className="empty-state-message">{welcomeMessage}</p>
              <div className="empty-state-avatar-wrap">
                <img src={pedroAvatar} alt="Pedro" className="empty-state-avatar" />
              </div>
            </div>
          </div>
        )}
        {Array.from({ length: messageHeadCount }, (_, i) => renderMessageRow(i))}
        {splitMessagesForToolCalls ? renderToolCallRows() : null}
        {splitMessagesForToolCalls
          ? messages.slice(messageHeadCount).map((_, j) => renderMessageRow(messageHeadCount + j))
          : null}
        {!splitMessagesForToolCalls && toolCalls.length > 0 ? renderToolCallRows() : null}

        {loading && (
          <div className="message-wrapper assistant">
            <div className="assistant-bubble-shell">
              <div className="message assistant assistant-reply-bubble">
                {streamingContent ? (
                  <MessageRenderer content={streamingContent} role="assistant" />
                ) : (
                  <div className="message-content typing">
                    Thinking<span className="thinking-dots"><span>.</span><span>.</span><span>.</span></span>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {attachments.length > 0 && (
        <div className="attachments-preview">
          {attachments.map((att, i) => (
            <div key={i} className="attachment-item">
              {att.type === 'image' ? (
                <img src={att.content} alt={att.name} className="attachment-thumb" />
              ) : att.type === 'folder-ref' ? (
                <span className="attachment-name" title={att.content}>
                  📁 {att.name}
                </span>
              ) : att.type === 'file-ref' ? (
                <span className="attachment-name" title={att.content}>
                  📄 {att.name}
                </span>
              ) : (
                <span className="attachment-name">📄 {att.name}</span>
              )}
              <button className="remove-attachment" onClick={() => removeAttachment(i)}>
                ×
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="input-area">
        <div className="composer-attach-wrap" ref={attachWrapRef}>
          <button
            type="button"
            className="file-attach-btn"
            title="Attach file or folder"
            aria-label="Attach file or folder"
            aria-expanded={attachMenuOpen}
            aria-haspopup="menu"
            disabled={loading}
            onClick={() => setAttachMenuOpen(o => !o)}
          >
            <img src={attachmentIcon} alt="" width={18} height={18} />
          </button>
          {attachMenuOpen && (
            <div className="attach-menu" role="menu" aria-label="Attachment type">
              <button
                type="button"
                role="menuitem"
                className="attach-menu-item"
                onClick={() => {
                  setAttachMenuOpen(false)
                  void handleSelectFile()
                }}
              >
                File…
              </button>
              <button
                type="button"
                role="menuitem"
                className="attach-menu-item"
                onClick={() => {
                  setAttachMenuOpen(false)
                  void handleSelectFolder()
                }}
              >
                Folder…
              </button>
            </div>
          )}
        </div>
        <textarea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type a message..."
          rows={1}
          disabled={loading}
        />
        <button
          type="button"
          className="send-btn"
          onClick={loading ? onStop : handleSend}
        >
          {loading ? 'Stop' : 'Send'}
        </button>
      </div>
    </div>
  )
}
