import { useState, useEffect, useLayoutEffect, useRef, memo, useMemo, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import hljs from 'highlight.js'
import mermaid from 'mermaid'
import { useTheme } from './ThemeContext'
import { fileService } from './services/wailsService'
import { looksLikeLocalFilesystemPath } from './utils/localPath'

// Memoized component to prevent re-renders from destroying the SVG
const MermaidDiagram = memo(function MermaidDiagram({ code, theme }: { code: string; theme: string }) {
  const [svg, setSvg] = useState<string | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    let cancelled = false

    const renderDiagram = async () => {
      try {
        mermaid.initialize({
          startOnLoad: false,
          theme: theme === 'dark' ? 'dark' : 'default',
        })
        const result = await mermaid.render(
          `mermaid-${Date.now()}`,
          code,
        )
        if (!cancelled) {
          setSvg(result.svg)
          setError(false)
        }
      } catch (err) {
        console.error('[Mermaid render error]', err)
        if (!cancelled) {
          setError(true)
        }
      }
    }

    // Small delay for Wails WebView
    const timer = setTimeout(renderDiagram, 50)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [code, theme])

  if (svg) {
    return (
      <div 
        className="mermaid-diagram" 
        dangerouslySetInnerHTML={{ __html: svg }} 
      />
    )
  }

  return (
    <div className="mermaid-diagram">
      <pre style={{ margin: 0 }}><code>{code}</code></pre>
    </div>
  )
}, (prevProps, nextProps) => {
  // Only re-render if code or theme changes
  return prevProps.code === nextProps.code && prevProps.theme === nextProps.theme
})

interface MessageRendererProps {
  content: string
  role: string
  isStreaming?: boolean
}

export default function MessageRenderer({
  content,
  role,
  isStreaming = false,
}: MessageRendererProps) {
  const { theme } = useTheme()
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [lightboxImage, setLightboxImage] = useState('')
  const codeRefs = useRef<HTMLElement[]>([])

  // Initialize mermaid once on mount.
  useEffect(() => {
    mermaid.initialize({
      startOnLoad: false,
      theme: theme === 'dark' ? 'dark' : 'default',
    })
  }, [])

  // Keep Mermaid theme in sync with the app theme.
  useEffect(() => {
    mermaid.initialize({
      startOnLoad: false,
      theme: theme === 'dark' ? 'dark' : 'default',
    })
  }, [theme])

  // Run syntax highlighting after render.
  useLayoutEffect(() => {
    // Skip while streaming to avoid issues with incomplete content
    if (isStreaming) return

    // Syntax highlighting
    codeRefs.current.forEach(el => {
      if (el) hljs.highlightElement(el)
    })

    // Reset refs after processing for next render
    return () => {
      codeRefs.current = []
    }
  }, [content, isStreaming])

  const isArtifact =
    content.trim().startsWith('<!DOCTYPE html>') ||
    content.trim().startsWith('<html') ||
    (content.includes('<script>') && content.includes('</html>'))

  const copyCode = useCallback((code: string) => navigator.clipboard.writeText(code), [])

  const components = useMemo(() => ({
    code({ className, children, ...props }: React.HTMLAttributes<HTMLElement> & { className?: string }) {
      const match = /language-(\w+)/.exec(className || '')
      const codeText = String(children).replace(/\n$/, '')
      const isMermaid = match?.[1] === 'mermaid'

      if (isMermaid) {
        if (isStreaming) {
          return (
            <pre className="mermaid-diagram">
              <code>{codeText}</code>
            </pre>
          )
        }
        return <MermaidDiagram code={codeText} theme={theme} />
      }

      if (match) {
        return (
          <div className="code-block">
            <div className="code-header">
              <span className="code-language">{match[1]}</span>
              <button className="copy-code-btn" onClick={() => copyCode(codeText)}>
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

      const inline = String(children).replace(/\n$/, '')
      if (looksLikeLocalFilesystemPath(inline)) {
        return (
          <a
            href="#"
            className="inline-code local-path-link"
            title="Open file or folder"
            onClick={e => {
              e.preventDefault()
              void fileService.openPath(inline).then(err => {
                if (err) {
                  console.warn('[OpenPath]', err)
                }
              })
            }}
          >
            {children}
          </a>
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
  }), [theme, isStreaming, copyCode])

  if (isStreaming) {
    return (
      <div className="message-container">
        <div className="message-body-wrap">
          <div className="markdown-content">
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontFamily: 'inherit' }}>
              {content}
            </pre>
          </div>
        </div>
      </div>
    )
  }

  const renderContent = () => {
    if (isArtifact && role === 'assistant') {
      return (
        <div className="artifact-preview">
          <iframe
            srcDoc={content}
            title="Artifact Preview"
            sandbox="allow-scripts"
          />
        </div>
      )
    }

    return (
      <div className="markdown-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={components as any}>
          {content}
        </ReactMarkdown>
      </div>
    )
  }

  return (
    <>
      <div className="message-container">
        <div className="message-body-wrap">
          {content ? renderContent() : <div className="markdown-content"><em>(empty message)</em></div>}
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
