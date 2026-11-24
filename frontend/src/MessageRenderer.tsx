import { useMemo, useCallback, memo } from 'react'
import { Streamdown } from 'streamdown'
import { code } from '@streamdown/code'
import { browserService, fileService } from './services/wailsService'
import { looksLikeLocalFilesystemPath } from './utils/localPath'
import { HtmlPreviewBlock } from './components/HtmlPreviewBlock'
import { MermaidBlock } from './components/diagram/MermaidBlock'
import { MessageImage } from './components/media/MessageImage'

interface MessageRendererProps {
  content: string
  role: string
  isStreaming?: boolean
}

const streamdownPlugins = {
  code,
  renderers: [
    { component: HtmlPreviewBlock, language: ['html', 'htm', 'svg'] },
    { component: MermaidBlock, language: ['mermaid'] },
  ],
}

function MessageRenderer({
  content,
  isStreaming = false,
}: MessageRendererProps) {
  const handleLinkClick = useCallback(
    (e: React.MouseEvent<HTMLAnchorElement>, href?: string) => {
      if (href && browserService.canOpenExternally) {
        e.preventDefault()
        browserService.openURL(href)
      }
    },
    [],
  )

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
      img: MessageImage,
      inlineCode({ children, className, ...props }: React.HTMLAttributes<HTMLElement>) {
        const inline = String(children).replace(/\n$/, '')
        if (looksLikeLocalFilesystemPath(inline)) {
          return (
            <code
              className={`local-path-link ${className ?? ''}`.trim()}
              title="Open file or folder"
              onClick={e => {
                e.preventDefault()
                void fileService.open(inline)
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

  return (
    <div className="message-container">
      {content ? (
        <Streamdown
          className="markdown-content"
          mode={isStreaming ? 'streaming' : 'static'}
          isAnimating={isStreaming}
          parseIncompleteMarkdown={isStreaming}
          plugins={streamdownPlugins}
          shikiTheme={['github-light', 'github-dark']}
          lineNumbers={false}
          components={components}
        >
          {content}
        </Streamdown>
      ) : (
        <div className="markdown-content">
          <em>(empty message)</em>
        </div>
      )}
    </div>
  )
}

export default memo(MessageRenderer)
