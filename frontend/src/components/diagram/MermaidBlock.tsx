import { useEffect, useState, type CSSProperties, type KeyboardEvent } from 'react'
import type { CustomRendererProps } from 'streamdown'
import { Check, Code, Copy, Maximize2, TriangleAlert, Workflow } from 'lucide-react'
import { useCopyToClipboard } from '../../hooks/useCopyToClipboard'
import { useTheme } from '../../ThemeContext'
import { Button } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { DiagramDownloadMenu } from './DiagramDownloadMenu'
import { diagramViewer } from './DiagramViewer'
import { renderMermaid, type RenderedDiagram } from './mermaidRenderer'

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/**
 * A ```mermaid fence in a message: the rendered diagram with copy, download and a
 * full-screen viewer. While the fence is still streaming it shows a placeholder.
 */
export function MermaidBlock({ code, isIncomplete }: CustomRendererProps) {
  const { theme } = useTheme()
  const source = code.trim()
  const [diagram, setDiagram] = useState<RenderedDiagram | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [showCode, setShowCode] = useState(false)
  const { copied, copy } = useCopyToClipboard()

  useEffect(() => {
    if (isIncomplete || !source) return
    let cancelled = false
    renderMermaid(source, theme).then(
      rendered => {
        if (cancelled) return
        setDiagram(rendered)
        setError(null)
      },
      err => {
        if (cancelled) return
        setDiagram(null)
        setError(errorMessage(err))
      },
    )
    return () => {
      cancelled = true
    }
  }, [source, theme, isIncomplete])

  function openViewer() {
    if (diagram) void diagramViewer.open({ payload: { diagram, code: source } })
  }

  function onPreviewKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key !== 'Enter' && e.key !== ' ') return
    e.preventDefault()
    openViewer()
  }

  const pending = isIncomplete || (!diagram && !error)
  const codeVisible = showCode || (!!error && !diagram)

  return (
    <div className="diagram-block" data-state={error ? 'error' : pending ? 'pending' : 'ready'}>
      <div className="diagram-block-header">
        <span className="diagram-block-label">
          <Workflow aria-hidden="true" />
          Diagram
          <span className="diagram-block-lang">mermaid</span>
        </span>
        <div className="diagram-block-actions">
          {diagram ? (
            <Button variant={showCode ? 'secondary' : 'ghost'} size="xs" onClick={() => setShowCode(v => !v)}>
              {showCode ? <Workflow aria-hidden="true" /> : <Code aria-hidden="true" />}
              {showCode ? 'Diagram' : 'Code'}
            </Button>
          ) : null}
          <Hint label={copied ? 'Copied' : 'Copy code'}>
            <Button variant="ghost" size="icon-xs" onClick={() => void copy(source)} disabled={!source}>
              {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
            </Button>
          </Hint>
          {diagram ? <DiagramDownloadMenu code={source} /> : null}
          {diagram ? (
            <Hint label="Open full screen">
              <Button variant="ghost" size="icon-xs" onClick={openViewer}>
                <Maximize2 aria-hidden="true" />
              </Button>
            </Hint>
          ) : null}
        </div>
      </div>

      {error && !diagram ? (
        <div className="diagram-error" role="alert">
          <TriangleAlert aria-hidden="true" />
          <span>
            <strong>This diagram has a syntax error.</strong> {error}
          </span>
        </div>
      ) : null}

      {codeVisible ? (
        <pre className="diagram-code">
          <code>{source}</code>
        </pre>
      ) : pending ? (
        <div className="diagram-pending" role="status">
          <span className="spinner" aria-hidden="true" />
          Drawing diagram…
        </div>
      ) : diagram ? (
        <div
          className="diagram-preview"
          role="button"
          tabIndex={0}
          aria-label="Open diagram full screen"
          onClick={openViewer}
          onKeyDown={onPreviewKeyDown}
          style={{ '--diagram-w': `${diagram.width}px`, '--diagram-h': `${diagram.height}px` } as CSSProperties}
        >
          <div className="diagram-preview-svg" dangerouslySetInnerHTML={{ __html: diagram.svg }} />
          <span className="diagram-preview-hint" aria-hidden="true">
            <Maximize2 />
            Open
          </span>
        </div>
      ) : null}
    </div>
  )
}
