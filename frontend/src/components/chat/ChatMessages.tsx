import { useEffect, useRef, useState } from 'react'
import { Check, Copy, FileText, Folder, Music, RefreshCw, Trash2 } from 'lucide-react'
import type { FileAttachment, ToolCall } from '../../hooks/useMessaging'
import MessageRenderer from '../../MessageRenderer'
import { fileService } from '../../services/wailsService'
import { Button } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { copyableImageSrc } from '../../utils/clipboard'
import { isAudioFilePath } from '../../utils/fileKinds'
import { lightbox } from '../ImageLightbox'
import { ToolCallGroup } from './ToolCallGroup'

function openPath(path: string) {
  void fileService.openPath(path).then(err => {
    if (err) console.warn('[OpenPath]', err)
  })
}

export function FileKindIcon({ folder, path }: { folder: boolean; path: string }) {
  if (folder) return <Folder aria-hidden="true" />
  if (isAudioFilePath(path)) return <Music aria-hidden="true" />
  return <FileText aria-hidden="true" />
}

function CopyAction({ onCopy, label = 'Copy', disabled }: { onCopy: () => Promise<unknown>; label?: string; disabled?: boolean }) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<number | undefined>(undefined)
  useEffect(() => () => window.clearTimeout(timer.current), [])

  return (
    <Hint label={copied ? 'Copied' : label}>
      <Button
        variant="ghost"
        size="icon-xs"
        disabled={disabled}
        data-copied={copied ? '' : undefined}
        onClick={async () => {
          try {
            await onCopy()
            setCopied(true)
            window.clearTimeout(timer.current)
            timer.current = window.setTimeout(() => setCopied(false), 1400)
          } catch {
            /* the caller reports failures */
          }
        }}
      >
        {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
      </Button>
    </Hint>
  )
}

/* ------------------------------------------------------------------ user */

interface UserMessageProps {
  content: string
  images?: string[]
  files?: FileAttachment[]
  busy: boolean
  onResend: () => void
  onDelete: () => void
}

export function UserMessage({ content, images, files, busy, onResend, onDelete }: UserMessageProps) {
  return (
    <article className="msg msg-user" aria-label="You">
      <div className="msg-user-stack">
        {images && images.length > 0 ? (
          <div className="msg-images">
            {images.map((src, j) => (
              <button
                key={`${j}-${src.slice(-24)}`}
                type="button"
                className="msg-image-thumb"
                onClick={() => void lightbox.open({ payload: src })}
                aria-label={`View attached image ${j + 1}`}
              >
                <img src={src} alt="" />
              </button>
            ))}
          </div>
        ) : null}
        {files && files.length > 0 ? (
          <div className="msg-files">
            {files.map(file => (
              <button
                key={file.path}
                type="button"
                className="file-chip"
                title={file.path}
                onClick={() => openPath(file.path)}
                aria-label={`Open ${file.type === 'folder' ? 'folder' : 'file'} ${file.name}`}
              >
                <FileKindIcon folder={file.type === 'folder'} path={file.path} />
                <span className="file-chip-name">{file.name}</span>
              </button>
            ))}
          </div>
        ) : null}
        {content ? (
          <div className="bubble bubble-user">
            <MessageRenderer content={content} role="user" />
          </div>
        ) : null}
        <div className="msg-actions" role="group" aria-label="Message actions">
          <Hint label="Resend">
            <Button variant="ghost" size="icon-xs" disabled={busy} onClick={onResend}>
              <RefreshCw aria-hidden="true" />
            </Button>
          </Hint>
          <Hint label="Delete from here">
            <Button variant="ghost" size="icon-xs" onClick={onDelete} className="msg-action-danger">
              <Trash2 aria-hidden="true" />
            </Button>
          </Hint>
        </div>
      </div>
    </article>
  )
}

/* ------------------------------------------------------------- assistant */

interface AssistantMessageProps {
  content: string
  toolCalls?: ToolCall[]
  /** Follows another Pedro message, so it sits closer to it. */
  continuation: boolean
  busy: boolean
  onCopy: () => Promise<unknown>
  onRegenerate: () => void
}

export function AssistantMessage({ content, toolCalls, continuation, busy, onCopy, onRegenerate }: AssistantMessageProps) {
  const hasText = content.trim().length > 0
  return (
    <article className="msg msg-assistant" aria-label="Pedro" data-continuation={continuation ? '' : undefined}>
      <div className="msg-main">
        {toolCalls && toolCalls.length > 0 ? <ToolCallGroup calls={toolCalls} /> : null}
        {hasText ? (
          <>
            <div className="bubble bubble-assistant">
              <MessageRenderer content={content} role="assistant" />
            </div>
            <div className="msg-actions" role="group" aria-label="Reply actions">
              <CopyAction onCopy={onCopy} label={copyableImageSrc(content) ? 'Copy image' : 'Copy'} />
              <Hint label="Regenerate">
                <Button variant="ghost" size="icon-xs" disabled={busy} onClick={onRegenerate}>
                  <RefreshCw aria-hidden="true" />
                </Button>
              </Hint>
            </div>
          </>
        ) : null}
      </div>
    </article>
  )
}

/* ------------------------------------------------------------- streaming */

interface StreamingMessageProps {
  content: string
  toolCalls: ToolCall[]
  continuation: boolean
}

export function StreamingMessage({ content, toolCalls, continuation }: StreamingMessageProps) {
  const hasText = content.trim().length > 0
  return (
    <article className="msg msg-assistant" aria-label="Pedro is replying" aria-busy="true" data-continuation={continuation ? '' : undefined}>
      <div className="msg-main">
        {toolCalls.length > 0 ? <ToolCallGroup calls={toolCalls} live /> : null}
        {hasText ? (
          <div className="bubble bubble-assistant" data-streaming="">
            <MessageRenderer content={content} role="assistant" isStreaming />
          </div>
        ) : (
          <div className="thinking" role="status">
            <span className="thinking-label">{toolCalls.length > 0 ? 'Working on it' : 'Thinking'}</span>
            <span className="thinking-dots" aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
          </div>
        )}
      </div>
    </article>
  )
}
