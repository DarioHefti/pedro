import { useState, useCallback } from 'react'
import type { CustomRendererProps } from 'streamdown'

function IconEye({ size = 14 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  )
}

function IconCode({ size = 14 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  )
}

function IconCopy({ size = 14 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  )
}

export function HtmlPreviewBlock({ code, language }: CustomRendererProps) {
  const [showPreview, setShowPreview] = useState(false)
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      /* ignore */
    }
  }, [code])

  return (
    <div className="preview-block">
      <div className="preview-block-header">
        <span className="preview-block-lang">{language}</span>
        <div className="preview-block-actions">
          <button
            type="button"
            className="preview-block-action-btn"
            onClick={handleCopy}
            title="Copy code"
            aria-label="Copy code"
          >
            {copied ? (
              <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            ) : (
              <IconCopy />
            )}
          </button>
          <button
            type="button"
            className="preview-block-action-btn preview-block-action-btn--primary"
            onClick={() => setShowPreview(v => !v)}
            title={showPreview ? 'Show code' : 'Preview output'}
            aria-label={showPreview ? 'Show code' : 'Preview output'}
          >
            {showPreview ? <IconCode /> : <IconEye />}
            <span>{showPreview ? 'Code' : 'Preview'}</span>
          </button>
        </div>
      </div>

      {showPreview ? (
        <div className="preview-frame-wrap">
          <iframe
            srcDoc={code}
            title={`${language.toUpperCase()} Preview`}
            sandbox="allow-scripts"
          />
        </div>
      ) : (
        <pre className="preview-block-code">
          <code>{code}</code>
        </pre>
      )}
    </div>
  )
}
