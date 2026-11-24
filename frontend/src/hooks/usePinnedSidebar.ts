import { useCallback, useLayoutEffect, useRef, useState, type RefObject } from 'react'
import type { ResizableController, ResizableLayoutCommitDetails } from '@ui-sdk/react/resizable'

interface SidebarPrefs {
  width: number
  /** Collapsed by the person, as opposed to squeezed shut by a narrow window. */
  collapsed: boolean
}

interface PinnedSidebarOptions {
  controller: ResizableController
  /** Id of the sidebar panel; it must be the first panel of the group. */
  panelId: string
  /** The Resizable group element, observed for window resizes. */
  groupRef: RefObject<HTMLElement | null>
  defaultWidth: number
  minWidth: number
  maxWidth: number
  /** localStorage key for the remembered width and collapsed state. */
  storageKey: string
}

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value))

function loadPrefs(key: string, defaultWidth: number, minWidth: number, maxWidth: number): SidebarPrefs {
  try {
    const saved = JSON.parse(localStorage.getItem(key) ?? 'null') as Partial<SidebarPrefs> | null
    const width = typeof saved?.width === 'number' ? saved.width : defaultWidth
    return { width: clamp(width, minWidth, maxWidth), collapsed: saved?.collapsed === true }
  } catch {
    return { width: defaultWidth, collapsed: false }
  }
}

/** Width the panels share: the group minus its resize handles. */
function sharedWidth(group: HTMLElement): number {
  let width = group.clientWidth
  group.querySelectorAll<HTMLElement>(':scope > [data-slot="resizable-handle"]').forEach(h => (width -= h.offsetWidth))
  return width
}

/**
 * Keeps a resizable sidebar at a fixed pixel width. Resizable keeps panel sizes as
 * percentages, so a wider window would widen the sidebar too, and a narrower one could
 * squeeze it shut. This puts it back at its width whenever the window resizes, so all the
 * extra room goes to the content. It remembers the width the person drags to and whether
 * they collapsed it.
 */
export function usePinnedSidebar({
  controller,
  panelId,
  groupRef,
  defaultWidth,
  minWidth,
  maxWidth,
  storageKey,
}: PinnedSidebarOptions) {
  const prefsRef = useRef<SidebarPrefs | null>(null)
  prefsRef.current ??= loadPrefs(storageKey, defaultWidth, minWidth, maxWidth)
  const prefs = prefsRef.current
  const [collapsed, setCollapsed] = useState(prefs.collapsed)
  /** Start at the remembered width, so the first paint is already right. */
  const [defaultSize] = useState(() => `${prefs.width}px` as const)

  const save = useCallback(() => {
    prefs.collapsed = controller.isCollapsed(panelId)
    setCollapsed(prefs.collapsed)
    try {
      localStorage.setItem(storageKey, JSON.stringify(prefs))
    } catch {
      /* storage unavailable: the layout just isn't remembered */
    }
  }, [prefs, controller, panelId, storageKey])

  /** Puts the sidebar back at its remembered width, unless the person collapsed it. */
  const pin = useCallback(() => {
    const group = groupRef.current
    if (!group || prefs.collapsed) return
    const shared = sharedWidth(group)
    if (shared <= 0) return
    // A window too narrow for it may have squeezed it shut.
    if (controller.isCollapsed(panelId)) controller.expand(panelId)
    controller.resize(panelId, (prefs.width / shared) * 100)
  }, [prefs, controller, panelId, groupRef])

  useLayoutEffect(() => {
    // Resizable sets its panels up in a microtask: collapse right after, before the first
    // paint, so a collapsed sidebar doesn't flash open.
    let active = true
    queueMicrotask(() => {
      if (!active) return
      if (prefs.collapsed) controller.collapse(panelId)
      setCollapsed(controller.isCollapsed(panelId))
    })

    // Resizable re-measures the group in the frame after it resizes. Pinning in that same
    // frame, after it, works against its new bounds, and nothing paints in between.
    const group = groupRef.current
    let frame = 0
    const observer = new ResizeObserver(() => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(pin)
    })
    if (group) observer.observe(group)
    return () => {
      active = false
      observer.disconnect()
      cancelAnimationFrame(frame)
    }
  }, [prefs, controller, panelId, groupRef, pin])

  const onLayoutChange = useCallback(() => setCollapsed(controller.isCollapsed(panelId)), [controller, panelId])

  /** Remembers what the person did with the handle: dragged, stepped with keys, or collapsed it. */
  const onLayoutCommit = useCallback(
    (layout: number[], details: ResizableLayoutCommitDetails) => {
      const { reason } = details
      if (reason !== 'drag' && reason !== 'step-key' && reason !== 'collapse') return
      const group = groupRef.current
      const restored = reason === 'collapse' && !controller.isCollapsed(panelId)
      if (!restored && group && !controller.isCollapsed(panelId)) {
        prefs.width = clamp(Math.round((layout[0] / 100) * sharedWidth(group)), minWidth, maxWidth)
      }
      save()
      // Reopened with a double-click or Enter on the handle: back to the remembered width.
      if (restored) pin()
    },
    [prefs, controller, panelId, groupRef, minWidth, maxWidth, pin, save],
  )

  const collapse = useCallback(() => {
    controller.collapse(panelId)
    save()
  }, [controller, panelId, save])

  const expand = useCallback(() => {
    controller.expand(panelId)
    save()
    pin()
  }, [controller, panelId, pin, save])

  const toggle = useCallback(() => {
    if (controller.isCollapsed(panelId)) expand()
    else collapse()
  }, [controller, panelId, expand, collapse])

  return { collapsed, defaultSize, onLayoutChange, onLayoutCommit, collapse, expand, toggle }
}
