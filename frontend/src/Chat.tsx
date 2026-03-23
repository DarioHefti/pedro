import { useState, useRef, useEffect, useCallback } from 'react'
import attachmentIcon from './assets/attachment.svg'
import { main } from '../wailsjs/go/models'
import { SelectFile } from '../wailsjs/go/main/App'
import { ToolCall } from './App'
import MessageRenderer from './MessageRenderer'

interface ChatProps {
  messages: main.Message[]
  toolCalls: ToolCall[]
  loading: boolean
  streamingContent: string
  messageImages: Map<number, string[]>
  onSend: (content: string, attachments?: { type: string; content: string; name: string }[]) => void
  onRegenerate: (index: number) => void
}

interface Attachment {
  type: 'text' | 'image' | 'file-ref'
  content: string  // for file-ref: full OS path; for text: file content; for image: data URL
  name: string
}

export default function Chat({ messages, toolCalls, loading, streamingContent, messageImages, onSend, onRegenerate }: ChatProps) {
  const [input, setInput] = useState('')
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [isDragging, setIsDragging] = useState(false)
  const [autoScroll, setAutoScroll] = useState(true)
  const [showJumpButtons, setShowJumpButtons] = useState(false)
  
  const bottomRef = useRef<HTMLDivElement>(null)
  const messagesRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (autoScroll && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages, toolCalls, loading, streamingContent, autoScroll])

  useEffect(() => {
    const handleScroll = () => {
      if (messagesRef.current) {
        const { scrollTop, scrollHeight, clientHeight } = messagesRef.current
        const atBottom = scrollHeight - scrollTop - clientHeight < 100
        setAutoScroll(atBottom)
        setShowJumpButtons(!atBottom)
      }
    }
    
    const msgDiv = messagesRef.current
    msgDiv?.addEventListener('scroll', handleScroll)
    return () => msgDiv?.removeEventListener('scroll', handleScroll)
  }, [])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleSend = () => {
    const content = input.trim()
    if (!content && attachments.length === 0) return
    
    const fullContent = attachments.length > 0 
      ? `${content}\n\n[Attached files: ${attachments.map(a => a.name).join(', ')}]`
      : content
    
    setInput('')
    setAttachments([])
    onSend(content, attachments)
  }

  // For images dropped/pasted: embed as data URL (fine, images are small)
  // For text files dropped: embed a truncated preview (we don't have the OS path here)
  const processFile = useCallback((file: File) => {
    if (file.type.startsWith('image/')) {
      const reader = new FileReader()
      reader.onload = () => {
        setAttachments(prev => [...prev, { type: 'image', content: reader.result as string, name: file.name }])
      }
      reader.readAsDataURL(file)
    } else {
      const reader = new FileReader()
      reader.onload = () => {
        const text = reader.result as string
        const MAX = 50_000
        const content = text.length > MAX
          ? text.slice(0, MAX) + `\n\n[File truncated — ${(text.length / 1024).toFixed(0)} KB total. Use the 📎 button to attach via path for full access.]`
          : text
        setAttachments(prev => [...prev, { type: 'text', content, name: file.name }])
      }
      reader.readAsText(file)
    }
  }, [])

  // Opens native OS file dialog and stores the full path as a file-ref attachment
  const handleSelectFile = useCallback(async () => {
    const path = await SelectFile()
    if (!path) return
    const name = path.split(/[\\/]/).pop() || path
    setAttachments(prev => [...prev, { type: 'file-ref', content: path, name }])
  }, [])

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
    const files = Array.from(e.dataTransfer.files)
    files.forEach(processFile)
  }

  const handlePaste = useCallback((e: ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return

    for (const item of Array.from(items)) {
      if (item.type.startsWith('image/')) {
        const file = item.getAsFile()
        if (file) processFile(file)
      }
    }
  }, [processFile])

  useEffect(() => {
    document.addEventListener('paste', handlePaste)
    return () => document.removeEventListener('paste', handlePaste)
  }, [handlePaste])

  const removeAttachment = (index: number) => {
    setAttachments(prev => prev.filter((_, i) => i !== index))
  }

  const jumpToBottom = () => {
    setAutoScroll(true)
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  const jumpToResponse = () => {
    const aiIndex = messages.findIndex(m => m.Role === 'assistant')
    if (aiIndex >= 0 && messagesRef.current) {
      const msgEl = messagesRef.current.children[aiIndex]
      msgEl?.scrollIntoView({ behavior: 'smooth' })
    }
  }

  const copyMessage = (content: string) => {
    navigator.clipboard.writeText(content)
  }

  const regenerateMessage = (index: number) => {
    onRegenerate(index)
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

      {showJumpButtons && (
        <div className="jump-buttons">
          <button onClick={jumpToResponse} title="Jump to response">↓ Response</button>
          <button onClick={jumpToBottom} title="Jump to bottom">↓ Bottom</button>
        </div>
      )}

      <div className="messages" ref={messagesRef}>
        {messages.map((msg, i) => {
          const imgs = messageImages.get(i)
          return (
            <div key={i} className={`message ${msg.Role}`}>
              {imgs && imgs.length > 0 && (
                <div className="message-image-previews">
                  {imgs.map((src, j) => (
                    <img key={j} src={src} alt={`Attached image ${j + 1}`} className="message-image-thumb" />
                  ))}
                </div>
              )}
              <MessageRenderer
                content={msg.Content || ''}
                role={msg.Role}
                onCopy={() => copyMessage(msg.Content || '')}
                onRegenerate={() => regenerateMessage(i)}
              />
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
          <div className="message assistant">
            {streamingContent ? (
              <MessageRenderer
                content={streamingContent}
                role="assistant"
                onCopy={() => {}}
                onRegenerate={() => {}}
              />
            ) : (
              <div className="message-content typing">Thinking...</div>
            )}
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
                <span className="attachment-name" title={att.content}>📁 {att.name}</span>
              ) : (
                <span className="attachment-name">📄 {att.name}</span>
              )}
              <button className="remove-attachment" onClick={() => removeAttachment(i)}>×</button>
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
        <button onClick={handleSend} disabled={loading}>
          {loading ? 'Stop' : 'Send'}
        </button>
      </div>
    </div>
  )
}