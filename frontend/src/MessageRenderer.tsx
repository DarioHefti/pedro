import { useState, useMemo, useCallback, memo, useEffect, useRef } from 'react'
import { Streamdown } from 'streamdown'
import { code } from '@streamdown/code'
import { mermaid } from '@streamdown/mermaid'
import { BrowserOpenURL } from '../wailsjs/runtime/runtime'
import { useTheme } from './ThemeContext'
import { fileService, isWailsDevStub } from './services/wailsService'
import { looksLikeLocalFilesystemPath } from './utils/localPath'
import { HtmlPreviewBlock } from './components/HtmlPreviewBlock'

interface MessageRendererProps {
  content: string
  role: string
  isStreaming?: boolean
}

const streamdownPlugins = {
  code,
  mermaid,
  renderers: [
    { component: HtmlPreviewBlock, language: ['html', 'htm', 'svg'] },
  ],
}

/** Language tag → common file extension used for downloads. */
const langToExt: Record<string, string> = {
  javascript: 'js',
  typescript: 'ts',
  python: 'py',
  rust: 'rs',
  go: 'go',
  java: 'java',
  kotlin: 'kt',
  swift: 'swift',
  ruby: 'rb',
  php: 'php',
  csharp: 'cs',
  c: 'c',
  cpp: 'cpp',
  'c++': 'cpp',
  shell: 'sh',
  bash: 'sh',
  zsh: 'zsh',
  powershell: 'ps1',
  html: 'html',
  css: 'css',
  scss: 'scss',
  json: 'json',
  yaml: 'yaml',
  yml: 'yml',
  xml: 'xml',
  sql: 'sql',
  markdown: 'md',
  mermaid: 'mmd',
  dockerfile: 'Dockerfile',
  vim: 'vim',
  lua: 'lua',
  perl: 'pl',
  r: 'r',
  scala: 'scala',
  groovy: 'groovy',
  dart: 'dart',
  elixir: 'ex',
  haskell: 'hs',
  clojure: 'clj',
  erlang: 'erl',
  ocaml: 'ml',
  fsharp: 'fs',
  solidity: 'sol',
}

function getSuggestedFilename(language: string): string {
  const ext = langToExt[language.toLowerCase()] || 'txt'
  return `download.${ext}`
}

function MessageRenderer({
  content,
  isStreaming = false,
}: MessageRendererProps) {
  const { theme } = useTheme()
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [lightboxImage, setLightboxImage] = useState('')
  const containerRef = useRef<HTMLDivElement>(null)

  const mermaidOptions = useMemo(
    () => ({
      config: {
        theme: (theme === 'dark' ? 'dark' : 'default') as 'dark' | 'default',
      },
    }),
    [theme],
  )

  /** Intercept Streamdown code-block download clicks and route through Wails so
   *  the native save dialog opens (WebView blobs / anchor clicks are unreliable). */
  useEffect(() => {
    if (isWailsDevStub) return

    const controller = new AbortController()

    const handler = async (e: MouseEvent) => {
      const target = e.target as HTMLElement
      const btn = target.closest(
        '[data-streamdown="code-block-download-button"]',
      ) as HTMLElement | null
      if (!btn) return

      e.stopPropagation()
      e.preventDefault()

      const block = btn.closest('[data-streamdown="code-block"]') as HTMLElement | null
      if (!block) return

      const pre = block.querySelector('pre')
      if (!pre) return

      const codeText = pre.textContent || ''
      const langEl = block.querySelector('span[class*="font-mono"]')
      const language = (langEl?.textContent || 'txt').trim()
      const filename = getSuggestedFilename(language)

      const err = await fileService.saveFile(filename, codeText)
      if (err) {
        console.warn('[SaveFile]', err)
      }
    }

    document.addEventListener('click', handler, { capture: true, signal: controller.signal })
    return () => controller.abort()
  }, [])

  const handleLinkClick = useCallback(
    (e: React.MouseEvent<HTMLAnchorElement>, href?: string) => {
      if (href && !isWailsDevStub) {
        e.preventDefault()
        BrowserOpenURL(href)
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

  return (
    <>
      <div className="message-container" ref={containerRef}>
        <div className="message-body-wrap">
          {content ? (
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
