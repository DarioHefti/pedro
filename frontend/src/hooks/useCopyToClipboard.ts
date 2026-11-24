import { useCallback, useEffect, useRef, useState } from 'react'

/** Copies text to the clipboard and exposes a short-lived `copied` flag. */
export function useCopyToClipboard(resetMs = 1400) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const copy = useCallback(async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => setCopied(false), resetMs)
    } catch { /* clipboard unavailable — ignore */ }
  }, [resetMs])

  return { copied, copy }
}
