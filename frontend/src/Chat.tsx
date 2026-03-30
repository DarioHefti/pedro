import { useState, useRef, useEffect, useLayoutEffect, useCallback, useMemo } from 'react'
import { OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime'
import attachmentIcon from './assets/attachment.svg'
import pedroAvatar from './assets/images/pedro.svg'
import type { Message, FileAttachment, ToolCall, Attachment } from './hooks/useMessaging'
import { fileService, type Persona } from './services/wailsService'
import MessageRenderer from './MessageRenderer'
import AssistantMessageActions from './AssistantMessageActions'

interface ChatProps {
  messages: Message[]
  toolCalls: ToolCall[]
  messageToolCalls: Map<number, ToolCall[]>
  /** True when this thread is showing an in-flight assistant stream. */
  loading: boolean
  /** True when any conversation has an active LLM stream (blocks sending elsewhere). */
  streamingBusy: boolean
  streamingContent: string
  messageImages: Map<number, string[]>
  messageFiles: Map<number, FileAttachment[]>
  /** selectedPersonaId is the DB persona id for this send (empty = none when 2+ personas). */
  onSend: (content: string, attachments: Attachment[] | undefined, selectedPersonaId: string) => void
  onStop: () => void
  /** selectedPersonaId is the DB persona id for this regeneration (read prompt from DB on backend). */
  onRegenerate: (index: number, selectedPersonaId: string) => void
  /** Opens the native OS file picker; resolves to the selected path or "". */
  onSelectFile: () => Promise<string>
  /** Opens the native folder picker; resolves to the selected path or "". */
  onSelectFolder: () => Promise<string>
  welcomeMessage: string
  personas: Persona[]
  activePersonaId: string
  onPersonaChange: (id: string) => void
  /** Increment to trigger focus on the input textarea. */
  focusTrigger?: number
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
  messageToolCalls,
  loading,
  streamingBusy,
  streamingContent,
  messageImages,
  messageFiles,
  onSend,
  onStop,
  onRegenerate,
  onSelectFile,
  onSelectFolder,
  welcomeMessage,
  personas,
  activePersonaId,
  onPersonaChange,
  focusTrigger,
}: ChatProps) {
  const [input, setInput] = useState('')
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [isDragging, setIsDragging] = useState(false)
  /** When true, new assistant content / stream keeps the viewport pinned to the bottom. */
  const [stickToBottom, setStickToBottom] = useState(true)
  const [showJumpButton, setShowJumpButton] = useState(false)
  const [attachMenuOpen, setAttachMenuOpen] = useState(false)
  const [personaMenuOpen, setPersonaMenuOpen] = useState(false)
  const [expandedToolCallSummaries, setExpandedToolCallSummaries] = useState<Set<number>>(
    () => new Set(),
  )

  const bottomRef = useRef<HTMLDivElement>(null)
  const messagesRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const inputAreaRef = useRef<HTMLDivElement>(null)
  const attachWrapRef = useRef<HTMLDivElement>(null)
  const personaWrapRef = useRef<HTMLDivElement>(null)
  /** Smooth jump-to-bottom animates scrollTop; ignore transient “away from bottom” during that window. */
  const suppressStickBreakRef = useRef(false)
  /**
   * Physical scrollTop while loading with !stickToBottom — synced each layout pass so we still have
   * the right value when the stream ends between chunks (no scroll event). Used to restore after the
   * streaming row is replaced (preserve scrollTop, not distance-from-bottom — the latter scrolls down
   * when the final bubble is taller than the stream, which feels like a jump).
   */
  const lastScrollTopWhileReadingRef = useRef(0)
  const prevLoadingRef = useRef(loading)

  const scrollMessagesToBottom = useCallback((behavior: ScrollBehavior) => {
    bottomRef.current?.scrollIntoView({ block: 'end', behavior })
  }, [])

  /** Match textarea height to content (min from CSS, max --composer-textarea-max-height). Grows upward; buttons sit outside .composer-input-shell. */
  const syncComposerTextareaHeight = useCallback(() => {
    const ta = inputRef.current
    if (!ta) return
    // Collapse first so scrollHeight reflects content, not the browser default (2 rows when rows is omitted).
    ta.style.height = '0px'
    const cs = getComputedStyle(ta)
    const minPx = parseFloat(cs.minHeight) || 0
    const maxRaw = cs.maxHeight
    const maxPx =
      maxRaw && maxRaw !== 'none' && !Number.isNaN(parseFloat(maxRaw))
        ? parseFloat(maxRaw)
        : Number.POSITIVE_INFINITY
    const next = Math.min(Math.max(ta.scrollHeight, minPx), maxPx)
    ta.style.height = `${next}px`
  }, [])

  useLayoutEffect(() => {
    syncComposerTextareaHeight()
  }, [input, syncComposerTextareaHeight])

  /** Keep jump-to-bottom offset in sync when the composer grows (uses --composer-body-height on :root). */
  useEffect(() => {
    const root = document.documentElement
    const el = inputAreaRef.current
    if (!el || typeof ResizeObserver === 'undefined') return

    const apply = () => {
      root.style.setProperty('--composer-body-height', `${el.offsetHeight}px`)
    }
    apply()
    const ro = new ResizeObserver(apply)
    ro.observe(el)
    return () => {
      ro.disconnect()
      root.style.removeProperty('--composer-body-height')
    }
  }, [])

  /** Focus composer on mount so the user can type without clicking (Wails: defer one frame after paint). */
  useEffect(() => {
    const id = requestAnimationFrame(() => {
      inputRef.current?.focus({ preventScroll: true })
    })
    return () => cancelAnimationFrame(id)
  }, [])

  /** Focus composer when focusTrigger changes (e.g. new chat clicked). */
  useEffect(() => {
    if (focusTrigger === undefined || focusTrigger === 0) return
    const id = requestAnimationFrame(() => {
      inputRef.current?.focus({ preventScroll: true })
    })
    return () => cancelAnimationFrame(id)
  }, [focusTrigger])

  /** User scrolled away from the tail → stop following. Following only resumes on Send or "jump to bottom". */
  useEffect(() => {
    const el = messagesRef.current
    if (!el) return

    const onScroll = () => {
      if (suppressStickBreakRef.current) return
      lastScrollTopWhileReadingRef.current = el.scrollTop
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

    if (loading && !stickToBottom) {
      lastScrollTopWhileReadingRef.current = el.scrollTop
    }

    const prevLoading = prevLoadingRef.current
    prevLoadingRef.current = loading
    const loadingEnded = prevLoading && !loading

    if (stickToBottom) {
      scrollMessagesToBottom('auto')
    } else if (loadingEnded) {
      // Keep viewport stable: only clamp if the thread shrank (browser would clamp scrollTop).
      const maxScroll = Math.max(0, el.scrollHeight - el.clientHeight)
      el.scrollTop = Math.min(Math.max(0, lastScrollTopWhileReadingRef.current), maxScroll)
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
    onSend(content, attachments, activePersonaId)
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

  const personaDisplayLabel = useMemo(() => {
    if (!activePersonaId) return 'None'
    const p = personas.find(x => String(x.ID) === activePersonaId)
    return p?.Name ?? 'None'
  }, [activePersonaId, personas])

  useEffect(() => {
    if (!personaMenuOpen) return
    const onDocMouseDown = (ev: MouseEvent) => {
      const el = personaWrapRef.current
      if (el && !el.contains(ev.target as Node)) setPersonaMenuOpen(false)
    }
    const onKeyDown = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') setPersonaMenuOpen(false)
    }
    document.addEventListener('mousedown', onDocMouseDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onDocMouseDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [personaMenuOpen])

  const selectPersona = useCallback(
    (id: string) => {
      onPersonaChange(id)
      setPersonaMenuOpen(false)
    },
    [onPersonaChange],
  )

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

  useEffect(() => {
    // Reset expanded summary state when thread content changes.
    setExpandedToolCallSummaries(new Set())
  }, [messages])

  const lastUserMessageIndex = messages.reduce(
    (acc, m, idx) => (m.Role === 'user' ? idx : acc),
    -1,
  )
  /** Tool rows belong after the latest user turn and before its assistant reply (incl. streaming). */
  const splitMessagesForToolCalls =
    toolCalls.length > 0 && lastUserMessageIndex >= 0

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
              <button
                key={j}
                type="button"
                className="message-file-chip"
                title={file.path}
                aria-label={`Open ${file.type === 'folder' ? 'folder' : 'file'}: ${file.name}`}
                onClick={() => {
                  void fileService.openPath(file.path).then(err => {
                    if (err) {
                      console.warn('[OpenPath]', err)
                    }
                  })
                }}
              >
                <span className="message-file-icon" aria-hidden>
                  {file.type === 'folder' ? '📁' : '📄'}
                </span>
                <span className="message-file-name">{file.name}</span>
              </button>
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
              onRegenerate={() => void onRegenerate(i, activePersonaId)}
              copyDisabled={!(msg.Content || '').trim()}
              regenerateDisabled={streamingBusy}
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

  function renderToolCallsBubble(
    calls: ToolCall[],
    collapsed: boolean,
    key?: string,
    onToggleCollapsed?: () => void,
  ) {
    if (calls.length === 0) return null
    return (
      <div className="message tool" key={key}>
        <div className="tool-call-card">
          <div className="tool-call-header">
            <span className="tool-call-icon">🔧</span>
            <span className="tool-call-label">tools used</span>
            <button
              type="button"
              className="tool-call-chevron-btn"
              aria-expanded={collapsed ? 'false' : 'true'}
              onClick={onToggleCollapsed}
            >
              <svg className="tool-call-chevron" width="12" height="12" viewBox="0 0 12 12" aria-hidden><path fill="currentColor" d="M2 4l4 4 4-4"/></svg>
            </button>
          </div>
          {!collapsed && (
            <div className="tool-call-lines">
              {calls.map((tc, i) => {
                const argsDisplay = toolCallArgDisplay(tc)
                return (
                  <div key={`tc-${i}`} className="tool-call-line">
                    <span className="tool-call-label">{tc.name}</span>
                    {argsDisplay ? <span className="tool-call-arg">{argsDisplay}</span> : null}
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    )
  }

  const inFlightToolCallsRow =
    toolCalls.length > 0 ? renderToolCallsBubble(toolCalls, false, 'tool-calls-in-flight') : null

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
        {messages.map((msg, i) => {
          const rows: JSX.Element[] = []
          const persistedToolCalls = messageToolCalls.get(i)
          if (msg.Role === 'assistant' && persistedToolCalls && persistedToolCalls.length > 0) {
            const isExpanded = expandedToolCallSummaries.has(i)
            const summaryRow = renderToolCallsBubble(
              persistedToolCalls,
              !isExpanded,
              `tool-calls-summary-${i}`,
              () =>
                setExpandedToolCallSummaries(prev => {
                  const next = new Set(prev)
                  if (next.has(i)) next.delete(i)
                  else next.add(i)
                  return next
                }),
            )
            if (summaryRow) rows.push(summaryRow)
          }
          rows.push(renderMessageRow(i))
          if (splitMessagesForToolCalls && i === lastUserMessageIndex && inFlightToolCallsRow) {
            rows.push(inFlightToolCallsRow)
          }
          return rows
        })}
        {!splitMessagesForToolCalls && inFlightToolCallsRow ? inFlightToolCallsRow : null}

        {loading && (
          <div className="message-wrapper assistant">
            <div className="assistant-bubble-shell">
              <div className="message assistant assistant-reply-bubble">
                {streamingContent ? (
                  <MessageRenderer content={streamingContent} role="assistant" isStreaming={true} />
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
                <button
                  type="button"
                  className="attachment-name attachment-name--open"
                  title={att.content}
                  onClick={() => {
                    void fileService.openPath(att.content).then(err => {
                      if (err) {
                        console.warn('[OpenPath]', err)
                      }
                    })
                  }}
                >
                  📁 {att.name}
                </button>
              ) : att.type === 'file-ref' ? (
                <button
                  type="button"
                  className="attachment-name attachment-name--open"
                  title={att.content}
                  onClick={() => {
                    void fileService.openPath(att.content).then(err => {
                      if (err) {
                        console.warn('[OpenPath]', err)
                      }
                    })
                  }}
                >
                  📄 {att.name}
                </button>
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

      <div className="composer-row" ref={inputAreaRef}>
        <div className="composer-attach-wrap" ref={attachWrapRef}>
          <button
            type="button"
            className="file-attach-btn"
            title="Attach file or folder"
            aria-label="Attach file or folder"
            aria-expanded={attachMenuOpen}
            aria-haspopup="menu"
            disabled={streamingBusy}
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
        {personas.length >= 2 && (
          <div className="composer-persona-wrap" ref={personaWrapRef}>
            <button
              type="button"
              className={`composer-persona-trigger${personaMenuOpen ? ' composer-persona-trigger--open' : ''}`}
              aria-label="Persona"
              title="Persona"
              aria-expanded={personaMenuOpen}
              aria-haspopup="listbox"
              disabled={streamingBusy}
              onClick={() => setPersonaMenuOpen(o => !o)}
            >
              <span className="composer-persona-trigger-label">{personaDisplayLabel}</span>
              <svg
                className="composer-persona-trigger-chevron"
                width="12"
                height="12"
                viewBox="0 0 12 12"
                aria-hidden
              >
                <path
                  d="M2.5 4.25L6 7.75l3.5-3.5"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </button>
            {personaMenuOpen && (
              <div className="composer-persona-menu" role="listbox" aria-label="Persona">
                <button
                  type="button"
                  role="option"
                  className={`composer-persona-option${activePersonaId === '' ? ' is-selected' : ''}`}
                  aria-selected={activePersonaId === ''}
                  onClick={() => selectPersona('')}
                >
                  None
                </button>
                {personas.map(p => {
                  const idStr = String(p.ID)
                  const selected = idStr === activePersonaId
                  return (
                    <button
                      key={p.ID}
                      type="button"
                      role="option"
                      className={`composer-persona-option${selected ? ' is-selected' : ''}`}
                      aria-selected={selected}
                      onClick={() => selectPersona(idStr)}
                    >
                      {p.Name}
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        )}
        <div className="composer-input-shell">
          <textarea
            ref={inputRef}
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message..."
            rows={1}
            disabled={streamingBusy}
          />
        </div>
        <button
          type="button"
          className="send-btn"
          onClick={streamingBusy ? onStop : handleSend}
        >
          {streamingBusy ? 'Stop' : 'Send'}
        </button>
      </div>
    </div>
  )
}
