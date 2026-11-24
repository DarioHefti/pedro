import { useState, useCallback } from 'react'
import type { CustomRendererProps } from 'streamdown'
import { Check, Code, Copy, Eye } from 'lucide-react'
import { Button } from '../ui/button'
import { Hint } from '../ui/hint'

export function HtmlPreviewBlock({ code, language }: CustomRendererProps) {
  const [showPreview, setShowPreview] = useState(false)
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      /* clipboard unavailable */
    }
  }, [code])

  return (
    <div className="preview-block">
      <div className="preview-block-header">
        <span className="preview-block-lang">{language}</span>
        <div className="preview-block-actions">
          <Hint label={copied ? 'Copied' : 'Copy code'}>
            <Button variant="ghost" size="icon-xs" onClick={handleCopy}>
              {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
            </Button>
          </Hint>
          <Button variant={showPreview ? 'secondary' : 'soft'} size="xs" onClick={() => setShowPreview(v => !v)}>
            {showPreview ? <Code aria-hidden="true" /> : <Eye aria-hidden="true" />}
            {showPreview ? 'Code' : 'Preview'}
          </Button>
        </div>
      </div>

      {showPreview ? (
        <div className="preview-frame-wrap">
          <iframe srcDoc={code} title={`${language.toUpperCase()} preview`} sandbox="allow-scripts" />
        </div>
      ) : (
        <pre className="preview-block-code">
          <code>{code}</code>
        </pre>
      )}
    </div>
  )
}
