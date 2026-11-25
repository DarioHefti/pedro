import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Copies to the clipboard and exposes a short-lived `copied` flag for the button's feedback.
 * `copy` writes text; `copyWith` runs any copy routine (e.g. an image) and flags it on success.
 * Failures leave the flag off: callers that need to report them do so in their routine.
 */
export function useCopyToClipboard(resetMs = 1400) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const copyWith = useCallback(
    async (routine: () => Promise<unknown>) => {
      try {
        await routine()
      } catch {
        return
      }
      setCopied(true)
      window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => setCopied(false), resetMs)
    },
    [resetMs],
  )

  const copy = useCallback((text: string) => copyWith(() => navigator.clipboard.writeText(text)), [copyWith])

  return { copied, copy, copyWith }
}
