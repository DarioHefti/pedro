import { Check, Copy } from 'lucide-react'
import { Button, type ButtonSize, type ButtonVariant } from '../ui/button'
import { useCopyToClipboard } from '../hooks/useCopyToClipboard'

interface CopyButtonProps {
  text: string | (() => string)
  label?: string
  copiedLabel?: string
  variant?: ButtonVariant
  size?: ButtonSize
  title?: string
}

/** Copies text and confirms with a check for a moment. */
export function CopyButton({
  text,
  label = 'Copy',
  copiedLabel = 'Copied',
  variant = 'ghost',
  size = 'xs',
  title,
}: CopyButtonProps) {
  const { copied, copy } = useCopyToClipboard()
  return (
    <Button
      variant={variant}
      size={size}
      title={title}
      data-copied={copied ? '' : undefined}
      onClick={() => void copy(typeof text === 'function' ? text() : text)}
    >
      {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
      {copied ? copiedLabel : label}
    </Button>
  )
}
