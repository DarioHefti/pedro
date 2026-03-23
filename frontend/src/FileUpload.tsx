import { useState, useRef, useCallback } from 'react'

interface FileUploadProps {
  onFileContent: (content: string, type: 'text' | 'image', name: string) => void
}

export default function FileUpload({ onFileContent }: FileUploadProps) {
  const [isDragging, setIsDragging] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const processFile = useCallback((file: File) => {
    if (file.type.startsWith('image/')) {
      const reader = new FileReader()
      reader.onload = () => {
        onFileContent(reader.result as string, 'image', file.name)
      }
      reader.readAsDataURL(file)
    } else if (isTextFile(file)) {
      const reader = new FileReader()
      reader.onload = () => {
        onFileContent(reader.result as string, 'text', file.name)
      }
      reader.readAsText(file)
    }
  }, [onFileContent])

  const isTextFile = (file: File): boolean => {
    const textTypes = ['text/plain', 'text/markdown', 'text/json', 'application/json', 'text/html', 'text/css', 'text/javascript', 'text/typescript']
    const codeExtensions = ['.ts', '.tsx', '.js', '.jsx', '.py', '.go', '.rs', '.java', '.c', '.cpp', '.h', '.cs', '.rb', '.php', '.swift', '.kt', '.sql', '.sh', '.bash', '.md', '.json', '.xml', '.yaml', '.yml']
    return textTypes.includes(file.type) || codeExtensions.some(ext => file.name.toLowerCase().endsWith(ext))
  }

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

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    files.forEach(processFile)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const handlePaste = useCallback((e: ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return

    for (const item of Array.from(items)) {
      if (item.type.startsWith('image/')) {
        const file = item.getAsFile()
        if (file) processFile(file)
      } else if (item.type.startsWith('text/')) {
        item.getAsString((text) => {
          onFileContent(text, 'text', 'Pasted text')
        })
      }
    }
  }, [processFile, onFileContent])

  return (
    <>
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*,.txt,.md,.json,.ts,.tsx,.js,.jsx,.py,.go,.rs,.java,.c,.cpp,.h,.cs,.rb,.php,.swift,.kt,.sql,.sh,.html,.css"
        multiple
        onChange={handleFileSelect}
        style={{ display: 'none' }}
      />
      {isDragging && (
        <div className="drop-overlay">
          <div className="drop-message">Drop files here to attach</div>
        </div>
      )}
      <button
        className="file-upload-btn"
        onClick={() => fileInputRef.current?.click()}
        title="Attach files"
      >
        📎
      </button>
    </>
  )
}

export { useCallback, useState, useRef }
export type { FileUploadProps }