import type { ComponentProps } from 'react'
import { Button as UiButton } from '@ui-sdk/react/button'

export type ButtonVariant =
  | 'primary'
  | 'secondary'
  | 'ghost'
  | 'subtle'
  | 'soft'
  | 'danger'
  | 'danger-ghost'
  /** Muted until hovered, then destructive: for delete buttons that sit in lists. */
  | 'danger-quiet'
  | 'link'
  /** Frosted control that sits on top of media (images, diagrams). */
  | 'overlay'

export type ButtonSize = 'xs' | 'sm' | 'md' | 'lg' | 'icon-xs' | 'icon-sm' | 'icon' | 'icon-lg'

export interface ButtonStyle {
  variant?: ButtonVariant
  size?: ButtonSize
  block?: boolean
  align?: 'start'
}

/**
 * Data attributes that give any element (a ui-sdk trigger, a close part, a
 * link) the design system's button look. Styles live in styles/ui.css.
 */
export function buttonStyle({ variant = 'secondary', size = 'md', block, align }: ButtonStyle = {}) {
  return {
    'data-btn': variant,
    'data-size': size,
    'data-block': block ? '' : undefined,
    'data-align': align,
  }
}

export type ButtonProps = ComponentProps<typeof UiButton> & ButtonStyle

export function Button({ variant, size, block, align, ...props }: ButtonProps) {
  return <UiButton {...buttonStyle({ variant, size, block, align })} {...props} />
}
