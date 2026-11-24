import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from 'react'

export interface PanZoomView {
  scale: number
  x: number
  y: number
}

interface Size {
  width: number
  height: number
}

const MIN_SCALE = 0.1
const MAX_SCALE = 8
const STEP = 1.25
/** Room kept around the content when it is fitted to the viewport. */
const FIT_PADDING = 48

const clampScale = (scale: number) => Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale))

/**
 * Pan and zoom for content of a known size inside a viewport element.
 * Drag pans, ⌘/Ctrl + wheel (and trackpad pinch) zooms around the pointer, plain wheel pans.
 * The content starts fitted and refits on resize until the person moves it.
 */
export function usePanZoom(viewportRef: RefObject<HTMLElement | null>, content: Size) {
  const [view, setView] = useState<PanZoomView>({ scale: 1, x: 0, y: 0 })
  const [dragging, setDragging] = useState(false)
  const touched = useRef(false)
  const drag = useRef<{ pointerX: number; pointerY: number; x: number; y: number } | null>(null)

  const viewportSize = useCallback((): Size => {
    const el = viewportRef.current
    return el ? { width: el.clientWidth, height: el.clientHeight } : { width: 0, height: 0 }
  }, [viewportRef])

  const fitView = useCallback((): PanZoomView => {
    const vp = viewportSize()
    if (!vp.width || !vp.height || !content.width || !content.height) return { scale: 1, x: 0, y: 0 }
    const scale = clampScale(
      Math.min((vp.width - FIT_PADDING * 2) / content.width, (vp.height - FIT_PADDING * 2) / content.height, 2),
    )
    return {
      scale,
      x: (vp.width - content.width * scale) / 2,
      y: (vp.height - content.height * scale) / 2,
    }
  }, [content.width, content.height, viewportSize])

  const fit = useCallback(() => {
    touched.current = false
    setView(fitView())
  }, [fitView])

  /** Zooms to `scale`, keeping the viewport point (px, py) fixed; defaults to the center. */
  const zoomTo = useCallback(
    (next: (scale: number) => number, px?: number, py?: number) => {
      touched.current = true
      setView(v => {
        const vp = viewportSize()
        const ox = px ?? vp.width / 2
        const oy = py ?? vp.height / 2
        const scale = clampScale(next(v.scale))
        const k = scale / v.scale
        return { scale, x: ox - (ox - v.x) * k, y: oy - (oy - v.y) * k }
      })
    },
    [viewportSize],
  )

  const zoomIn = useCallback(() => zoomTo(s => s * STEP), [zoomTo])
  const zoomOut = useCallback(() => zoomTo(s => s / STEP), [zoomTo])
  const actualSize = useCallback(() => zoomTo(() => 1), [zoomTo])

  const panBy = useCallback((dx: number, dy: number) => {
    touched.current = true
    setView(v => ({ ...v, x: v.x + dx, y: v.y + dy }))
  }, [])

  // Fit on open and whenever the viewport resizes, until the person takes over.
  useEffect(() => {
    const el = viewportRef.current
    if (!el) return
    setView(fitView())
    const observer = new ResizeObserver(() => {
      if (!touched.current) setView(fitView())
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [viewportRef, fitView])

  // Wheel needs a non-passive listener to keep the page from scrolling or zooming.
  useEffect(() => {
    const el = viewportRef.current
    if (!el) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      if (e.ctrlKey || e.metaKey) {
        const rect = el.getBoundingClientRect()
        const factor = Math.exp(-e.deltaY * 0.0025)
        zoomTo(s => s * factor, e.clientX - rect.left, e.clientY - rect.top)
      } else {
        panBy(-e.deltaX, -e.deltaY)
      }
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [viewportRef, zoomTo, panBy])

  const onPointerDown = useCallback(
    (e: ReactPointerEvent<HTMLElement>) => {
      if (e.button !== 0) return
      drag.current = { pointerX: e.clientX, pointerY: e.clientY, x: view.x, y: view.y }
      e.currentTarget.setPointerCapture(e.pointerId)
      setDragging(true)
    },
    [view.x, view.y],
  )

  const onPointerMove = useCallback((e: ReactPointerEvent<HTMLElement>) => {
    const start = drag.current
    if (!start) return
    touched.current = true
    setView(v => ({ ...v, x: start.x + e.clientX - start.pointerX, y: start.y + e.clientY - start.pointerY }))
  }, [])

  const onPointerUp = useCallback((e: ReactPointerEvent<HTMLElement>) => {
    drag.current = null
    if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId)
    setDragging(false)
  }, [])

  const onDoubleClick = useCallback(
    (e: ReactMouseEvent<HTMLElement>) => {
      const rect = e.currentTarget.getBoundingClientRect()
      zoomTo(s => s * 2, e.clientX - rect.left, e.clientY - rect.top)
    },
    [zoomTo],
  )

  return {
    view,
    dragging,
    zoomIn,
    zoomOut,
    actualSize,
    fit,
    panBy,
    canZoomIn: view.scale < MAX_SCALE,
    canZoomOut: view.scale > MIN_SCALE,
    handlers: { onPointerDown, onPointerMove, onPointerUp, onPointerCancel: onPointerUp, onDoubleClick },
  }
}
