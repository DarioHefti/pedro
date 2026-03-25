import { useState, useEffect, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import hljs from 'highlight.js'
import mermaid from 'mermaid'
import { useTheme } from './ThemeContext'

interface MessageRendererProps {
  content: string
  role: string
}

export default function MessageRenderer({
  content,
  role,
}: MessageRendererProps) {
  const { theme } = useTheme()
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [lightboxImage, setLightboxImage] = useState('')
  // These are reset to [] on each render cycle (before children push into them)
  // so that refs don't accumulate across re-renders.
  const codeRefs = useRef<HTMLElement[]>([])
  const mermaidRefs = useRef<HTMLDivElement[]>([])

  // Keep Mermaid theme in sync with the app theme.
  useEffect(() => {
    mermaid.initialize({
      startOnLoad: false,
      theme: theme === 'dark' ? 'dark' : 'default',
    })
  }, [theme])

  // Run syntax highlighting and Mermaid rendering whenever content changes.
  useEffect(() => {
    // Reset and re-highlight; refs were populated during this render.
    codeRefs.current.forEach(el => {
      if (el) hljs.highlightElement(el)
    })

    mermaidRefs.current.forEach(async (el, i) => {
      if (el?.dataset.mermaid) {
        try {
          const { svg } = await mermaid.render(
            `mermaid-${i}-${Date.now()}`,
            el.dataset.mermaid,
          )
          el.innerHTML = svg
        } catch {
          el.textContent = el.dataset.mermaid ?? ''
        }
      }
    })
  }, [content])

  const isArtifact =
    content.trim().startsWith('<!DOCTYPE html>') ||
    content.trim().startsWith('<html') ||
    (content.includes('<script>') && content.includes('</html>'))

  const copyCode = (code: string) => navigator.clipboard.writeText(code)

  const components = {
    code({ className, children, ...props }: React.HTMLAttributes<HTMLElement> & { className?: string }) {
      const match = /language-(\w+)/.exec(className || '')
      const code = String(children).replace(/\n$/, '')
      const isMermaid = match?.[1] === 'mermaid'

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
              <button className="copy-code-btn" onClick={() => copyCode(code)}>
                Copy
              </button>
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

      return (
        <code className="inline-code" {...props}>
          {children}
        </code>
      )
    },
    img({ src, alt }: React.ImgHTMLAttributes<HTMLImageElement>) {
      if (src?.startsWith('data:image')) {
        return (
          <img
            src={src}
            alt={alt ?? 'Image'}
            className="message-image"
            onClick={() => {
              setLightboxImage(src)
              setLightboxOpen(true)
            }}
          />
        )
      }
      return <img src={src} alt={alt ?? 'Image'} className="message-image" />
    },
    a({ href, children }: React.AnchorHTMLAttributes<HTMLAnchorElement>) {
      return (
        <a href={href} target="_blank" rel="noopener noreferrer">
          {children}
        </a>
      )
    },
  }

  // Reset ref arrays before each render so they don't grow unboundedly.
  codeRefs.current = []
  mermaidRefs.current = []

  return (
    <>
      <div
        className="message-container"
      >
        <div className="message-body-wrap">
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
              <ReactMarkdown remarkPlugins={[remarkGfm]} components={components as any}>
                {content}
              </ReactMarkdown>
            </div>
          )}
        </div>

      </div>

      {lightboxOpen && (
        <div className="lightbox" onClick={() => setLightboxOpen(false)}>
          <img src={lightboxImage} alt="Full size" />
        </div>
      )}
    </>
  )
}
