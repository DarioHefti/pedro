import type { Ref } from 'react'
import { ArrowUp, FileText, Folder, Paperclip, Square, X } from 'lucide-react'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@ui-sdk/react/menu'
import { Textarea } from '@ui-sdk/react/textarea'
import type { Attachment } from '../../hooks/useMessaging'
import { fileService } from '../../services/wailsService'
import { Button, buttonStyle } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { FileKindIcon } from './ChatMessages'

interface ComposerProps {
  value: string
  onValueChange: (value: string) => void
  attachments: Attachment[]
  onRemoveAttachment: (index: number) => void
  onSend: () => void
  onStop: () => void
  /** Any reply is streaming: sending waits, the send button becomes Stop. */
  busy: boolean
  onAttachFile: () => void
  onAttachFolder: () => void
  inputRef: Ref<HTMLTextAreaElement>
}

function AttachmentChip({ attachment, onRemove }: { attachment: Attachment; onRemove: () => void }) {
  const isRef = attachment.type === 'file-ref' || attachment.type === 'folder-ref'
  return (
    <div className="attachment" data-kind={attachment.type}>
      {attachment.type === 'image' ? (
        <img src={attachment.content} alt={attachment.name} className="attachment-thumb" />
      ) : (
        <button
          type="button"
          className="attachment-body"
          title={isRef ? attachment.content : attachment.name}
          disabled={!isRef}
          onClick={() => {
            if (!isRef) return
            void fileService.openPath(attachment.content).then(err => {
              if (err) console.warn('[OpenPath]', err)
            })
          }}
        >
          <span className="attachment-icon">
            {attachment.type === 'text' ? (
              <FileText aria-hidden="true" />
            ) : (
              <FileKindIcon folder={attachment.type === 'folder-ref'} path={attachment.content} />
            )}
          </span>
          <span className="attachment-text">
            <span className="attachment-name">{attachment.name}</span>
            <span className="attachment-kind">
              {attachment.type === 'folder-ref' ? 'Folder' : attachment.type === 'file-ref' ? 'File path' : 'Text'}
            </span>
          </span>
        </button>
      )}
      <button type="button" className="attachment-remove" onClick={onRemove} aria-label={`Remove ${attachment.name}`}>
        <X aria-hidden="true" />
      </button>
    </div>
  )
}

export function Composer({
  value,
  onValueChange,
  attachments,
  onRemoveAttachment,
  onSend,
  onStop,
  busy,
  onAttachFile,
  onAttachFolder,
  inputRef,
}: ComposerProps) {
  const canSend = !busy && (value.trim().length > 0 || attachments.length > 0)

  return (
    <div className="composer" data-busy={busy ? '' : undefined}>
      {/* The input's frame: its own element, so the attach button can sit outside it. */}
      <span className="composer-box" aria-hidden="true" />

      {attachments.length > 0 ? (
        <div className="composer-attachments">
          {attachments.map((att, i) => (
            <AttachmentChip key={`${att.type}:${att.content.slice(0, 64)}:${att.name}`} attachment={att} onRemove={() => onRemoveAttachment(i)} />
          ))}
        </div>
      ) : null}

      <div className="composer-lead">
        <Menu
          onAction={action => {
            if (action === 'file') onAttachFile()
            else if (action === 'folder') onAttachFolder()
          }}
        >
          <Hint label="Attach a file or folder — or paste and drop">
            <MenuTrigger disabled={busy} className="composer-attach" {...buttonStyle({ variant: 'secondary', size: 'icon' })}>
              <Paperclip aria-hidden="true" />
            </MenuTrigger>
          </Hint>
          <MenuContent side="top" align="start" sideOffset={8}>
            <MenuItem value="file">
              <FileText aria-hidden="true" />
              File…
              <span className="menu-item-hint">by path</span>
            </MenuItem>
            <MenuItem value="folder">
              <Folder aria-hidden="true" />
              Folder…
              <span className="menu-item-hint">by path</span>
            </MenuItem>
          </MenuContent>
        </Menu>
      </div>

      <Textarea
        ref={inputRef}
        className="composer-input"
        value={value}
        onValueChange={next => onValueChange(next)}
        onSubmit={() => {
          if (canSend) onSend()
        }}
        submitShortcut="enter"
        autoResize
        rows={1}
        placeholder="Message Pedro…"
        aria-label="Message Pedro"
      />

      <div className="composer-trail">
        {busy ? (
          <Hint label="Stop generating" shortcut="escape">
            <Button variant="secondary" size="icon-sm" onClick={onStop} className="composer-stop">
              <Square aria-hidden="true" />
            </Button>
          </Hint>
        ) : (
          <Hint label="Send message" shortcut="enter">
            <Button variant="primary" size="icon-sm" onClick={onSend} disabled={!canSend} focusableWhenDisabled className="composer-send">
              <ArrowUp aria-hidden="true" />
            </Button>
          </Hint>
        )}
      </div>
    </div>
  )
}
