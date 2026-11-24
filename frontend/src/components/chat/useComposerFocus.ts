import { useCallback, useEffect, useRef } from 'react'

const INTERACTIVE = 'input, textarea, select, button, a, [contenteditable], [tabindex]'
/** Clicks inside any open dialog (lightbox, diagram viewer, …) belong to that dialog. */
const DIALOG = '[data-slot="dialog-content"], [data-slot="alert-dialog-content"], [role="dialog"]'

/**
 * Keeps the composer focused: on demand (`focusTrigger`), when the window regains focus, and when
 * the person clicks empty space. Paused while a dialog owns focus.
 */
export function useComposerFocus(focusTrigger: number | undefined, paused: boolean) {
  const inputRef = useRef<HTMLTextAreaElement>(null)

  /**
   * WebView2 can ignore a single focus() right after window focus events, so retry for ~300ms and
   * stop as soon as focus lands.
   */
  const focusComposer = useCallback((retries = 6) => {
    const attempt = (remaining: number) => {
      const el = inputRef.current
      if (!el) return
      el.focus({ preventScroll: true })
      if (document.activeElement === el || remaining <= 0) return
      setTimeout(() => attempt(remaining - 1), 50)
    }
    requestAnimationFrame(() => attempt(retries))
  }, [])

  useEffect(() => {
    if (focusTrigger === undefined || focusTrigger === 0) return
    focusComposer()
  }, [focusTrigger, focusComposer])

  /**
   * In WebView2, focus() is ignored until the webview itself has OS focus. Redirect a click on
   * empty space to the composer, and refocus when the window comes back.
   */
  useEffect(() => {
    if (paused) return
    const onPointerUp = (e: PointerEvent) => {
      const target = e.target as HTMLElement
      if (target.closest(INTERACTIVE) || target.closest(DIALOG)) return
      const selection = window.getSelection()
      if (selection && selection.toString().length > 0) return
      setTimeout(() => inputRef.current?.focus({ preventScroll: true }), 0)
    }
    const onWindowFocus = () => focusComposer(6)
    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') focusComposer(6)
    }

    document.addEventListener('pointerup', onPointerUp)
    window.addEventListener('focus', onWindowFocus)
    document.addEventListener('visibilitychange', onVisibilityChange)
    focusComposer(10)
    return () => {
      document.removeEventListener('pointerup', onPointerUp)
      window.removeEventListener('focus', onWindowFocus)
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  }, [paused, focusComposer])

  return { inputRef, focusComposer }
}
