import { useState, useMemo, useCallback, memo } from 'react'
import { Streamdown } from 'streamdown'
import { code } from '@streamdown/code'
import { mermaid } from '@streamdown/mermaid'
import { BrowserOpenURL } from '../wailsjs/runtime/runtime'
import { useTheme } from './ThemeContext'
import { fileService, isWailsDevStub } from './services/wailsService'
import { looksLikeLocalFilesystemPath } from './utils/localPath'

interface MessageRendererProps {
  content: string
  role: string
  isStreaming?: boolean
}

const streamdownPlugins = { code, mermaid }

function MessageRenderer({
  content,
  role,
  isStreaming = false,
}: MessageRendererProps) {
  const { theme } = useTheme()
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [lightboxImage, setLightboxImage] = useState('')

  const isArtifact =
    content.trim().startsWith('<!DOCTYPE html>') ||
    content.trim().startsWith('<html') ||
    (content.includes('<script>') && content.includes('</html>'))

  const mermaidOptions = useMemo(
    () => ({
      config: {
        theme: (theme === 'dark' ? 'dark' : 'default') as 'dark' | 'default',
      },
    }),
    [theme],
  )

  const handleLinkClick = useCallback((e: React.MouseEvent<HTMLAnchorElement>, href?: string) => {
    if (href && !isWailsDevStub) {
      e.preventDefault()
      BrowserOpenURL(href)
    }
  }, [])

  const components = useMemo(
    () => ({
      a({ href, children, className, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) {
        return (
          <a
            href={href}
            className={className}
            target="_blank"
            rel="noopener noreferrer"
            onClick={e => handleLinkClick(e, href)}
            {...props}
          >
            {children}
          </a>
        )
      },
      img({ src, alt, className, ...props }: React.ImgHTMLAttributes<HTMLImageElement>) {
        if (src?.startsWith('data:image')) {
          return (
            <img
              src={src}
              alt={alt ?? 'Image'}
              className={`message-image ${className ?? ''}`.trim()}
              onClick={() => {
                setLightboxImage(src)
                setLightboxOpen(true)
              }}
              {...props}
            />
          )
        }
        return (
          <img
            src={src}
            alt={alt ?? 'Image'}
            className={`message-image ${className ?? ''}`.trim()}
            {...props}
          />
        )
      },
      inlineCode({ children, className, ...props }: React.HTMLAttributes<HTMLElement>) {
        const inline = String(children).replace(/\n$/, '')
        if (looksLikeLocalFilesystemPath(inline)) {
          return (
            <code
              className={`local-path-link ${className ?? ''}`.trim()}
              title="Open file or folder"
              onClick={e => {
                e.preventDefault()
                void fileService.openPath(inline).then(err => {
                  if (err) {
                    console.warn('[OpenPath]', err)
                  }
                })
              }}
              {...props}
            >
              {children}
            </code>
          )
        }
        return (
          <code className={className} {...props}>
            {children}
          </code>
        )
      },
    }),
    [handleLinkClick],
  )

  const renderMarkdown = () => (
    <Streamdown
      className="markdown-content"
      mode={isStreaming ? 'streaming' : 'static'}
      isAnimating={isStreaming}
      parseIncompleteMarkdown={isStreaming}
      plugins={streamdownPlugins}
      mermaid={mermaidOptions}
      shikiTheme={['github-light', 'github-dark']}
      lineNumbers={false}
      components={components}
    >
      {content}
    </Streamdown>
  )

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

    return renderMarkdown()
  }

  return (
    <>
      <div className="message-container">
        <div className="message-body-wrap">
          {content ? (
            renderContent()
          ) : (
            <div className="markdown-content">
              <em>(empty message)</em>
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

export default memo(MessageRenderer)
