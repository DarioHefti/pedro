import type { ReactElement, ReactNode } from 'react'
import { Tooltip, TooltipContent, TooltipShortcut, TooltipTrigger } from '@ui-sdk/react/tooltip'

type Side = 'top' | 'bottom' | 'left' | 'right'

interface HintProps {
  label: ReactNode
  /** Shown as a key hint, like "mod+k". It doesn't bind the keys. */
  shortcut?: string
  side?: Side
  disabled?: boolean
  /** The trigger: any element or ui-sdk part that forwards props and ref. */
  children: ReactElement
}

/** Names an icon button (or describes a labelled one) on hover and keyboard focus. */
export function Hint({ label, shortcut, side = 'top', disabled, children }: HintProps) {
  return (
    <Tooltip shortcut={shortcut} delay={380} closeDelay={60} disabled={disabled}>
      <TooltipTrigger render={children} />
      {/* A span keeps hints valid inside paragraphs, e.g. on an image that sits in running text. */}
      <TooltipContent side={side} sideOffset={7} collisionPadding={8} render={<span />}>
        {label}
        {shortcut ? <TooltipShortcut /> : null}
      </TooltipContent>
    </Tooltip>
  )
}
