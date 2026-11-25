import { useCallback, useEffect, useState, type DragEvent } from 'react'
import type { Attachment } from '../../hooks/messageModel'
import { fileDropService } from '../../services/wailsService'

const MAX_TEXT_ATTACHMENT = 50_000

function baseName(path: string): string {
  return path.split(/[\\/]/).pop() || path
}

function fileRef(path: string, kind: 'file-ref' | 'folder-ref' = 'file-ref'): Attachment {
  return { type: kind, content: path, name: baseName(path) }
}

/** Reads a browser File into an attachment: images as data URLs, anything else as (capped) text. */
function readFile(file: File): Promise<Attachment> {
  return new Promise(resolve => {
    const reader = new FileReader()
    if (file.type.startsWith('image/')) {
      reader.onload = () => resolve({ type: 'image', content: reader.result as string, name: file.name || 'Pasted image' })
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
      resolve({ type: 'text', content, name: file.name })
    }
    reader.readAsText(file)
  })
}

interface AttachmentSources {
  selectFile: () => Promise<string>
  selectFolder: () => Promise<string>
  /** Called after a picker added an attachment, e.g. to refocus the composer. */
  onPicked: () => void
}

/**
 * The composer's pending attachments, from every source: the file and folder pickers, pasted
 * images, files dropped on the window (native paths in Wails, File objects in a browser).
 */
export function useComposerAttachments({ selectFile, selectFolder, onPicked }: AttachmentSources) {
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [isDragging, setIsDragging] = useState(false)

  const add = useCallback((...items: Attachment[]) => setAttachments(prev => [...prev, ...items]), [])
  const remove = useCallback((index: number) => setAttachments(prev => prev.filter((_, i) => i !== index)), [])
  const clear = useCallback(() => setAttachments([]), [])

  const addFile = useCallback((file: File) => void readFile(file).then(att => add(att)), [add])

  const pickFile = useCallback(async () => {
    const path = await selectFile()
    if (!path) return
    add(fileRef(path))
    onPicked()
  }, [selectFile, add, onPicked])

  const pickFolder = useCallback(async () => {
    const path = await selectFolder()
    if (!path) return
    add(fileRef(path, 'folder-ref'))
    onPicked()
  }, [selectFolder, add, onPicked])

  // Native drops deliver absolute paths, which tools can read in full.
  useEffect(
    () =>
      fileDropService.onDrop(paths => {
        setIsDragging(false)
        add(...paths.map(path => fileRef(path)))
      }),
    [add],
  )

  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      for (const item of Array.from(e.clipboardData?.items ?? [])) {
        if (!item.type.startsWith('image/')) continue
        const file = item.getAsFile()
        if (file) addFile(file)
      }
    }
    document.addEventListener('paste', onPaste)
    return () => document.removeEventListener('paste', onPaste)
  }, [addFile])

  const dropHandlers = {
    onDragOver: (e: DragEvent) => {
      e.preventDefault()
      setIsDragging(true)
    },
    onDragLeave: (e: DragEvent) => {
      e.preventDefault()
      if (e.currentTarget.contains(e.relatedTarget as Node | null)) return
      setIsDragging(false)
    },
    onDrop: (e: DragEvent) => {
      e.preventDefault()
      setIsDragging(false)
      // With native drops (Wails OnFileDrop) the paths arrive separately; File objects are the browser fallback.
      if (fileDropService.isNative) return
      for (const file of Array.from(e.dataTransfer?.files ?? [])) addFile(file)
    },
  }

  return { attachments, remove, clear, pickFile, pickFolder, isDragging, dropHandlers }
}
