import { useCallback, useState, type JSX } from 'react'
import { ArrowDown, Upload } from 'lucide-react'
import { useShortcut } from '@ui-sdk/react/shortcuts'
import { stripFileAnnotations, type Attachment, type ThreadState } from '../../hooks/messageModel'
import type { ChatStats } from '../../hooks/useChatStats'
import type { StreamBuffer } from '../../hooks/useMessaging'
import { copyMessageContent } from '../../utils/clipboard'
import { toast } from '../../ui/toaster'
import { AssistantMessage, StreamingMessage, UserMessage } from './ChatMessages'
import { ChatHeader } from './ChatHeader'
import { Composer } from './Composer'
import { EmptyState, type SuggestionAttach } from './EmptyState'
import { useComposerAttachments } from './useComposerAttachments'
import { useComposerFocus } from './useComposerFocus'
import { useThreadScroll } from './useThreadScroll'

interface ChatProps {
  title: string
  thread: ThreadState
  /** The reply streaming into this chat, or null. */
  stream: StreamBuffer | null
  /** Any conversation has an active stream (blocks sending elsewhere). */
  streamingBusy: boolean
  onSend: (content: string, attachments: Attachment[]) => void
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
  stats: ChatStats
  onViewDetails?: () => void
  sidebarCollapsed: boolean
  onShowSidebar: () => void
  onNewChat: () => void
  onOpenPalette: () => void
}

const NO_TOOL_CALLS: StreamBuffer['toolCalls'] = []

export default function Chat({
  title,
  thread,
  stream,
  streamingBusy,
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
  stats,
  onViewDetails,
  sidebarCollapsed,
  onShowSidebar,
  onNewChat,
  onOpenPalette,
}: ChatProps) {
  const { messages } = thread
  const streaming = stream !== null
  const [input, setInput] = useState('')

  const { inputRef, focusComposer } = useComposerFocus(focusTrigger, composerFocusPaused)
  const refocus = useCallback(() => focusComposer(), [focusComposer])
  const files = useComposerAttachments({ selectFile: onSelectFile, selectFolder: onSelectFolder, onPicked: refocus })
  const scroll = useThreadScroll({
    messages,
    streaming,
    streamContent: stream?.content ?? '',
    streamToolCalls: stream?.toolCalls ?? NO_TOOL_CALLS,
    conversationID: currentConvID,
  })

  useShortcut(streamingBusy ? 'escape' : null, () => onStop(), { allowInInput: true, description: 'Stop generating' })

  const handleSend = () => {
    const content = input.trim()
    if (streamingBusy || (!content && files.attachments.length === 0)) return
    scroll.follow()
    setInput('')
    files.clear()
    onSend(content, files.attachments)
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

  const { pickFile, pickFolder } = files
  const handlePickSuggestion = useCallback(
    (prompt: string, attach?: SuggestionAttach) => {
      setInput(prompt)
      if (attach === 'file') void pickFile()
      else if (attach === 'folder') void pickFolder()
      else focusComposer()
    },
    [pickFile, pickFolder, focusComposer],
  )

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
          images={thread.images.get(i)}
          files={thread.files.get(i)}
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
          toolCalls={thread.toolCalls.get(i)}
          continuation={prevRole === 'assistant'}
          busy={streamingBusy}
          onCopy={() => handleCopyMessage(content)}
          onRegenerate={() => onRegenerate(i)}
        />,
      )
    }
    prevRole = msg.Role
  })

  const isEmpty = messages.length === 0 && !streaming

  return (
    <main className="chat" {...files.dropHandlers} data-empty={isEmpty ? '' : undefined}>
      <ChatHeader
        title={isEmpty ? 'New chat' : title}
        sidebarCollapsed={sidebarCollapsed}
        onShowSidebar={onShowSidebar}
        onNewChat={onNewChat}
        onOpenPalette={onOpenPalette}
        stats={currentConvID != null ? stats : undefined}
        onViewDetails={onViewDetails}
      />

      <div className="thread" ref={scroll.threadRef}>
        {isEmpty ? (
          <EmptyState welcomeMessage={welcomeMessage} onPick={handlePickSuggestion} />
        ) : (
          <div className="thread-inner">
            {rows}
            {stream ? (
              <StreamingMessage content={stream.content} toolCalls={stream.toolCalls} continuation={prevRole === 'assistant'} />
            ) : null}
          </div>
        )}
      </div>

      <div className="composer-dock">
        {scroll.showJumpButton && messages.length > 0 ? (
          <button type="button" className="jump-latest" onClick={scroll.jumpToLatest}>
            <ArrowDown aria-hidden="true" />
            {streaming ? 'New messages' : 'Jump to latest'}
          </button>
        ) : null}
        <Composer
          value={input}
          onValueChange={setInput}
          attachments={files.attachments}
          onRemoveAttachment={files.remove}
          onSend={handleSend}
          onStop={onStop}
          busy={streamingBusy}
          onAttachFile={() => void pickFile()}
          onAttachFolder={() => void pickFolder()}
          inputRef={inputRef}
        />
      </div>

      {files.isDragging ? (
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
