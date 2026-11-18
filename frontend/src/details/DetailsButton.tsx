import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { useCopyToClipboard } from './useCopyToClipboard'

type Variant = 'ghost' | 'outline' | 'danger' | 'danger-solid' | 'icon'
type Size = 'sm' | 'md'

interface DetailsButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  children: ReactNode
}

/** Shared button for the Request Details modal; all looks live in `.rd-btn` variants. */
export function DetailsButton({
  variant = 'ghost',
  size = 'sm',
  className,
  children,
  type = 'button',
  ...rest
}: DetailsButtonProps) {
  const classes = ['rd-btn', `rd-btn--${variant}`, `rd-btn--${size}`, className].filter(Boolean).join(' ')
  return (
    <button type={type} className={classes} {...rest}>
      {children}
    </button>
  )
}

interface CopyButtonProps {
  text: string | (() => string)
  label?: string
  copiedLabel?: string
  variant?: Variant
  size?: Size
  title?: string
}

export function CopyButton({
  text,
  label = 'Copy',
  copiedLabel = 'Copied',
  variant = 'ghost',
  size = 'sm',
  title,
}: CopyButtonProps) {
  const { copied, copy } = useCopyToClipboard()
  return (
    <DetailsButton
      variant={variant}
      size={size}
      title={title}
      data-state={copied ? 'copied' : undefined}
      onClick={() => void copy(typeof text === 'function' ? text() : text)}
    >
      {copied ? copiedLabel : label}
    </DetailsButton>
  )
}
