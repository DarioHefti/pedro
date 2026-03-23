import { useState, useRef, useEffect, useCallback } from 'react'
import attachmentIcon from './assets/attachment.svg'
import pedroAvatar from './assets/images/pedro.svg'
import type { Message, FileAttachment, ToolCall, Attachment } from './hooks/useMessaging'
import MessageRenderer from './MessageRenderer'

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
  welcomeMessage: string
}

/**
 * Strip [File: ...] / [Path: ...] annotations that buildLLMContent appends to
 * user messages before sending to the LLM. These are stored verbatim in the DB
 * but should never be shown in the chat UI.
 */
function stripFileAnnotations(content: string): string {
  return content.replace(/\n\n\[File: [\s\S]*$/, '')
}

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
  welcomeMessage,
}: ChatProps) {
  const [input, setInput] = useState('')
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [isDragging, setIsDragging] = useState(false)
  const [autoScroll, setAutoScroll] = useState(true)
  const [showJumpButton, setShowJumpButton] = useState(false)
  const [userHasScrolledUp, setUserHasScrolledUp] = useState(false)

  const bottomRef = useRef<HTMLDivElement>(null)
  const messagesRef = useRef<HTMLDivElement>(null)
  const prevScrollTop = useRef<number>(0)
  const prevStreamingContent = useRef<string>('')

  useEffect(() => {
    if (autoScroll && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages, toolCalls, loading, streamingContent, autoScroll])

  useEffect(() => {
    const handleScroll = () => {
      const el = messagesRef.current
      if (!el) return
      const { scrollTop, scrollHeight, clientHeight } = el
      const atBottom = scrollHeight - scrollTop - clientHeight < 100

      if (scrollTop < prevScrollTop.current - 5) {
        setUserHasScrolledUp(true)
        setAutoScroll(false)
      }
      prevScrollTop.current = scrollTop

      if (atBottom && !userHasScrolledUp) setAutoScroll(true)
      setShowJumpButton(!atBottom)
    }

    const el = messagesRef.current
    el?.addEventListener('scroll', handleScroll)
    return () => el?.removeEventListener('scroll', handleScroll)
  }, [userHasScrolledUp])

  // Re-enable auto-scroll once streaming finishes.
  useEffect(() => {
    if (!streamingContent && prevStreamingContent.current) {
      setUserHasScrolledUp(false)
    }
    prevStreamingContent.current = streamingContent
  }, [streamingContent])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleSend = () => {
    const content = input.trim()
    if (!content && attachments.length === 0) return
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
              `\n\n[File truncated — ${(text.length / 1024).toFixed(0)} KB total. Use the 📎 button to attach via path for full access.]`
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
    Array.from(e.dataTransfer.files).forEach(processFile)
  }

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
    setAutoScroll(true)
    setUserHasScrolledUp(false)
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
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
          <div className="drop-message">Drop files here to attach</div>
        </div>
      )}

      {showJumpButton && (
        <div className="jump-buttons">
          <button onClick={jumpToBottom} title="Jump to bottom">
            ↓ Bottom
          </button>
        </div>
      )}

      <div className="messages" ref={messagesRef}>
        {messages.length === 0 && !loading && (
          <div className="empty-state">
            <p className="empty-state-message">{welcomeMessage}</p>
          </div>
        )}
        {messages.map((msg, i) => {
          const imgs = messageImages.get(i)
          const files = messageFiles.get(i)
          return (
            <div key={i} className={`message-wrapper ${msg.Role}`}>
              {msg.Role === 'assistant' && (
                <img src={pedroAvatar} alt="Pedro" className="message-avatar" />
              )}
              <div className={`message ${msg.Role}`}>
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
                      <span className="message-file-icon">📄</span>
                      <span className="message-file-name">{file.name}</span>
                    </div>
                  ))}
                </div>
              )}
              <MessageRenderer
                content={msg.Role === 'user' ? stripFileAnnotations(msg.Content || '') : (msg.Content || '')}
                role={msg.Role}
              />
              {msg.Role === 'assistant' && (
                <div className="message-actions">
                  <button onClick={() => navigator.clipboard.writeText(msg.Content || '')} title="Copy">
                    Copy
                  </button>
                  <button onClick={() => onRegenerate(i)} title="Regenerate">
                    Regenerate
                  </button>
                </div>
              )}
            </div>
            </div>
          )
        })}

        {toolCalls.map((tc, i) => (
          <div key={`tc-${i}`} className="message tool">
            <div className="tool-call-card">
              <span className="tool-call-icon">🔧</span>
              <span className="tool-call-label">{tc.name}</span>
            </div>
          </div>
        ))}

        {loading && (
          <div className="message-wrapper">
            <img src={pedroAvatar} alt="Pedro" className="message-avatar" />
            <div className="message assistant">
              {streamingContent ? (
                <MessageRenderer
                  content={streamingContent}
                  role="assistant"
                />
              ) : (
                <div className="message-content typing">Thinking...</div>
              )}
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
              ) : att.type === 'file-ref' ? (
                <span className="attachment-name" title={att.content}>
                  📁 {att.name}
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
        <button
          className="file-attach-btn"
          title="Attach file (opens file picker — large files are referenced by path)"
          onClick={handleSelectFile}
          disabled={loading}
        >
          <img src={attachmentIcon} alt="Attach file" width={20} height={20} />
        </button>
        <textarea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type a message... (Ctrl+V to paste images)"
          rows={1}
          disabled={loading}
        />
        <button onClick={loading ? onStop : handleSend}>
          {loading ? 'Stop' : 'Send'}
        </button>
      </div>
    </div>
  )
}
