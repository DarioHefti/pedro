import { useState, useEffect, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import hljs from 'highlight.js'
import mermaid from 'mermaid'
import { main } from '../wailsjs/go/models'

interface MessageRendererProps {
  content: string
  role: string
  onCopy: () => void
  onRegenerate?: () => void
}

export default function MessageRenderer({ content, role, onCopy, onRegenerate }: MessageRendererProps) {
  const [showActions, setShowActions] = useState(false)
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [lightboxImage, setLightboxImage] = useState('')
  const codeRefs = useRef<HTMLElement[]>([])
  const mermaidRefs = useRef<HTMLDivElement[]>([])

  useEffect(() => {
    mermaid.initialize({ startOnLoad: false, theme: 'default' })
  }, [])

  useEffect(() => {
    codeRefs.current.forEach((codeEl, i) => {
      if (codeEl) {
        hljs.highlightElement(codeEl)
      }
    })

    mermaidRefs.current.forEach(async (el, i) => {
      if (el && el.dataset.mermaid) {
        try {
          const { svg } = await mermaid.render(`mermaid-${i}-${Date.now()}`, el.dataset.mermaid)
          el.innerHTML = svg
        } catch (e) {
          el.textContent = el.dataset.mermaid || ''
        }
      }
    })
  }, [content])

  const isArtifact = content.trim().startsWith('<!DOCTYPE html>') || 
                     content.trim().startsWith('<html') ||
                     (content.includes('<script>') && content.includes('</html>'))

  const openLightbox = (img: string) => {
    setLightboxImage(img)
    setLightboxOpen(true)
  }

  const copyCode = (code: string) => {
    navigator.clipboard.writeText(code)
  }

  const components = {
    code({ node, className, children, ...props }: any) {
      const match = /language-(\w+)/.exec(className || '')
      const code = String(children).replace(/\n$/, '')
      const isMermaid = match && match[1] === 'mermaid'

      if (isMermaid) {
        return (
          <div 
            ref={el => { if (el) mermaidRefs.current.push(el) }}
            data-mermaid={code}
            className="mermaid-diagram"
          />
        )
      }

      if (match) {
        return (
          <div className="code-block">
            <div className="code-header">
              <span className="code-language">{match[1]}</span>
              <button className="copy-code-btn" onClick={() => copyCode(code)}>Copy</button>
            </div>
            <pre>
              <code 
                ref={el => { if (el) codeRefs.current.push(el) }}
                className={`hljs language-${match[1]}`}
                {...props}
              >
                {children}
              </code>
            </pre>
          </div>
        )
      }

      return <code className="inline-code" {...props}>{children}</code>
    },
    img({ src, alt }: any) {
      if (src?.startsWith('data:image')) {
        return (
          <img 
            src={src} 
            alt={alt || 'Image'} 
            className="message-image"
            onClick={() => openLightbox(src)}
          />
        )
      }
      return <img src={src} alt={alt || 'Image'} className="message-image" />
    },
    a({ href, children }: any) {
      return <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>
    }
  }

  return (
    <>
      <div 
        className="message-wrapper"
        onMouseEnter={() => setShowActions(true)}
        onMouseLeave={() => setShowActions(false)}
      >
        {showActions && (
          <div className="message-actions">
            <button onClick={onCopy} title="Copy">Copy</button>
            {role === 'assistant' && onRegenerate && (
              <button onClick={onRegenerate} title="Regenerate">Regenerate</button>
            )}
          </div>
        )}
        {isArtifact && role === 'assistant' ? (
          <div className="artifact-preview">
            <iframe 
              srcDoc={content}
              title="Artifact Preview"
              sandbox="allow-scripts"
            />
          </div>
        ) : (
          <div className="markdown-content">
            <ReactMarkdown 
              remarkPlugins={[remarkGfm]}
              components={components}
            >
              {content}
            </ReactMarkdown>
          </div>
        )}
      </div>
      
      {lightboxOpen && (
        <div className="lightbox" onClick={() => setLightboxOpen(false)}>
          <img src={lightboxImage} alt="Full size" />
        </div>
      )}
    </>
  )
}