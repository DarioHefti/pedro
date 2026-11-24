import { ChevronRight, Wrench } from 'lucide-react'
import { Collapsible, CollapsiblePanel, CollapsibleTrigger } from '@ui-sdk/react/collapsible'
import type { ToolCall } from '../../hooks/messageModel'
import { summarizeTools, toolCallArgDisplay, toolMeta } from './toolCalls'

interface ToolCallGroupProps {
  calls: ToolCall[]
  /** The calls of a reply that's still streaming: open, with the last step running. */
  live?: boolean
}

const MAX_CHIPS = 3

export function ToolCallGroup({ calls, live = false }: ToolCallGroupProps) {
  if (calls.length === 0) return null
  const labels = summarizeTools(calls)
  const extra = labels.length - MAX_CHIPS

  return (
    <Collapsible defaultOpen={live} className="tool-group" data-live={live ? '' : undefined}>
      <CollapsibleTrigger className="tool-group-trigger">
        <span className="tool-group-icon" aria-hidden="true">
          {live ? <span className="spinner" /> : <Wrench />}
        </span>
        <span className="tool-group-label">
          {live ? 'Working' : 'Used'} {calls.length} {calls.length === 1 ? 'tool' : 'tools'}
        </span>
        <span className="tool-group-chips" aria-hidden="true">
          {labels.slice(0, MAX_CHIPS).map(label => (
            <span key={label} className="tool-chip">
              {label}
            </span>
          ))}
          {extra > 0 ? <span className="tool-chip tool-chip-more">+{extra}</span> : null}
        </span>
        <ChevronRight className="tool-group-chevron" aria-hidden="true" />
      </CollapsibleTrigger>
      <CollapsiblePanel>
        <ol className="tool-steps">
          {calls.map((tc, i) => {
            const meta = toolMeta(tc.name)
            const Icon = meta.icon
            const arg = toolCallArgDisplay(tc)
            const running = live && i === calls.length - 1
            return (
              <li key={tc.id ?? `${tc.name}-${i}`} className="tool-step" data-running={running ? '' : undefined}>
                <span className="tool-step-icon" aria-hidden="true">
                  {running ? <span className="spinner" /> : <Icon />}
                </span>
                <span className="tool-step-name">{meta.label}</span>
                {arg ? (
                  <span className="tool-step-arg" title={arg}>
                    {arg}
                  </span>
                ) : null}
              </li>
            )
          })}
        </ol>
      </CollapsiblePanel>
    </Collapsible>
  )
}
