import { useMemo } from 'react'
import { CopyButton } from './DetailsButton'
import { MAX_RENDER, humanSize, prettyJson } from './payload'

/* The raw view is plain text, so allow a much larger slice than message bodies. */
const RAW_RENDER_LIMIT = MAX_RENDER * 8

export function RawJsonView({ json }: { json: string }) {
  const pretty = useMemo(() => prettyJson(json), [json])
  const truncated = pretty.length > RAW_RENDER_LIMIT

  return (
    <div className="rd-raw">
      <div className="rd-toolbar">
        <span className="rd-muted-note">{humanSize(pretty.length)} · pretty-printed</span>
        <CopyButton text={pretty} label="Copy JSON" />
      </div>
      <pre className="rd-code rd-code--raw">{truncated ? pretty.slice(0, RAW_RENDER_LIMIT) : pretty}</pre>
      {truncated && (
        <div className="rd-msg-trunc">
          Showing first {humanSize(RAW_RENDER_LIMIT)} — use Copy JSON to grab everything.
        </div>
      )}
    </div>
  )
}
