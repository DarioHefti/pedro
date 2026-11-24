import { mermaid } from '@streamdown/mermaid'
import type { Theme } from '../../ThemeContext'

/** A rendered Mermaid diagram: its SVG markup, the element id inside it, and its natural size. */
export interface RenderedDiagram {
  svg: string
  id: string
  width: number
  height: number
}

interface RenderOptions {
  /**
   * Draw labels as SVG text instead of HTML. Needed for files and images: HTML labels
   * (foreignObject) block canvas export and don't show in most SVG editors.
   */
  forExport?: boolean
}

let renderCount = 0
/** Mermaid's configuration is global, so renders run one at a time with their own config. */
let queue: Promise<unknown> = Promise.resolve()

/** The size the diagram declares in its root viewBox. */
function naturalSize(svg: string): { width: number; height: number } {
  const rootTag = svg.slice(0, svg.indexOf('>') + 1)
  const viewBox = /viewBox="([^"]+)"/.exec(rootTag)?.[1].trim().split(/[\s,]+/).map(Number)
  if (viewBox && viewBox.length === 4 && viewBox[2] > 0 && viewBox[3] > 0) {
    return { width: viewBox[2], height: viewBox[3] }
  }
  return { width: 800, height: 600 }
}

let swatchContext: CanvasRenderingContext2D | null = null

/** A theme colour token as #rrggbb: Mermaid's theme engine only understands hex colours. */
function tokenHex(name: string): string {
  swatchContext ??= document.createElement('canvas').getContext('2d')
  const value = getComputedStyle(document.documentElement).getPropertyValue(`--${name}`).trim()
  if (!swatchContext || !value) return '#808080'
  swatchContext.fillStyle = '#808080'
  swatchContext.fillStyle = `hsl(${value})`
  return String(swatchContext.fillStyle)
}

/** Mermaid's base theme, coloured from the app's tokens so diagrams match the app in both themes. */
function themeVariables(dark: boolean, fontFamily: string) {
  return {
    darkMode: dark,
    fontFamily,
    background: tokenHex('surface'),
    primaryColor: tokenHex('accent'),
    primaryTextColor: tokenHex('foreground'),
    primaryBorderColor: tokenHex('brand'),
    secondaryColor: tokenHex('surface-2'),
    tertiaryColor: tokenHex('muted'),
    textColor: tokenHex('foreground'),
    lineColor: tokenHex('muted-foreground'),
    edgeLabelBackground: tokenHex('surface'),
    clusterBkg: tokenHex('surface-2'),
    clusterBorder: tokenHex('border-strong'),
    noteBkgColor: tokenHex('surface-2'),
    noteTextColor: tokenHex('foreground'),
    noteBorderColor: tokenHex('border-strong'),
  }
}

type Rgb = [number, number, number]

function parseRgb(color: string): Rgb | null {
  const m = /^rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)(?:,\s*([\d.]+))?\)$/.exec(color.trim())
  if (!m || (m[4] !== undefined && Number(m[4]) < 0.5)) return null
  return [Number(m[1]), Number(m[2]), Number(m[3])]
}

function luminance([r, g, b]: Rgb): number {
  const lin = (v: number) => (v / 255 <= 0.03928 ? v / 255 / 12.92 : ((v / 255 + 0.055) / 1.055) ** 2.4)
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}

const contrast = (a: Rgb, b: Rgb) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

/** Label text needs this much contrast with its box; below it the label gets a readable ink. */
const MIN_LABEL_CONTRAST = 4.5

/**
 * Boxes the author coloured themselves (`style A fill:#bbf`, `classDef`) keep the theme's
 * text colour, which can vanish on them, like light text on a pale box in dark mode. Gives
 * such labels whichever ink reads on their box. Works on the styled result, so it catches
 * every way a fill can be set.
 */
function readableLabels(svg: string): string {
  const host = document.createElement('div')
  host.className = 'visually-hidden'
  host.innerHTML = svg
  document.body.append(host)
  try {
    const root = host.querySelector('svg')
    if (!root) return svg
    const inkRgb = [tokenHex('ink-on-light'), tokenHex('ink-on-dark')].map(
      hex => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16)) as Rgb,
    )
    let changed = false
    for (const node of root.querySelectorAll<SVGGElement>('g.node')) {
      const shape = node.querySelector('.label-container') ?? node.querySelector('rect, polygon, circle, ellipse, path')
      const fill = shape ? parseRgb(getComputedStyle(shape).fill) : null
      const labels = [...node.querySelectorAll<HTMLElement | SVGElement>('.nodeLabel, text')]
      if (!fill || labels.length === 0) continue
      const label = labels[0]
      const text = parseRgb(getComputedStyle(label)[label instanceof SVGElement ? 'fill' : 'color'])
      if (text && contrast(fill, text) >= MIN_LABEL_CONTRAST) continue
      const best = inkRgb.reduce((a, b) => (contrast(fill, b) > contrast(fill, a) ? b : a))
      const ink = `rgb(${best.join(', ')})`
      for (const el of labels) el.style.setProperty(el instanceof SVGElement ? 'fill' : 'color', ink, 'important')
      changed = true
    }
    return changed ? new XMLSerializer().serializeToString(root) : svg
  } finally {
    host.remove()
  }
}

/** Longest wait for the web font before drawing anyway (e.g. offline). */
const FONT_WAIT_MS = 2000

/**
 * Mermaid sizes every label box by measuring its text, so the font must be ready first:
 * measuring with a fallback font clips labels once the real font arrives.
 */
async function fontReady(fontFamily: string): Promise<void> {
  if (!document.fonts) return
  const timeout = new Promise<void>(resolve => window.setTimeout(resolve, FONT_WAIT_MS))
  const loaded = Promise.all([document.fonts.load(`16px ${fontFamily}`), document.fonts.ready]).then(
    () => undefined,
    () => undefined,
  )
  await Promise.race([loaded, timeout])
}

async function render(code: string, theme: Theme, { forExport = false }: RenderOptions): Promise<RenderedDiagram> {
  const fontFamily = getComputedStyle(document.documentElement).getPropertyValue('--font-sans').trim()
  await fontReady(fontFamily || 'sans-serif')
  const instance = mermaid.getMermaid({
    theme: 'base',
    themeVariables: themeVariables(theme === 'dark', fontFamily || 'sans-serif'),
    fontFamily: fontFamily || 'sans-serif',
    securityLevel: 'strict',
    htmlLabels: !forExport,
    flowchart: { htmlLabels: !forExport },
  })
  renderCount += 1
  const id = `pedro-diagram-${renderCount}`
  try {
    const { svg: drawn } = await instance.render(id, code)
    const svg = readableLabels(drawn)
    return { svg, id, ...naturalSize(svg) }
  } finally {
    // Mermaid leaves its scratch element behind when parsing fails.
    document.getElementById(`d${id}`)?.remove()
  }
}

/** Renders Mermaid source with the app's theme and font. Rejects with Mermaid's parse error. */
export function renderMermaid(code: string, theme: Theme, options: RenderOptions = {}): Promise<RenderedDiagram> {
  const result = queue.then(() => render(code, theme, options))
  queue = result.catch(() => undefined)
  return result
}

/**
 * The same diagram under new element ids, so a second copy (the viewer) can sit in the
 * page next to the first without their styles and arrow markers colliding.
 */
export function withFreshIds(diagram: RenderedDiagram, suffix: string): RenderedDiagram {
  const id = `${diagram.id}-${suffix}`
  return { ...diagram, id, svg: diagram.svg.split(diagram.id).join(id) }
}
