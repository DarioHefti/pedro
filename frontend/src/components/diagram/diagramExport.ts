import { fileService } from '../../services/wailsService'
import type { Theme } from '../../ThemeContext'
import { toast } from '../../ui/toaster'
import { copyPngToClipboard } from '../../utils/clipboard'
import { saveWithFeedback } from '../media/mediaActions'
import { renderMermaid, type RenderedDiagram } from './mermaidRenderer'

export type DiagramFormat = 'svg' | 'png' | 'mmd'

/** PNG exports are rendered at this multiple of the diagram's natural size. */
const PNG_SCALE = 2

/** A file name from the diagram type, e.g. "sequence-diagram" for `sequenceDiagram`. */
export function diagramFileBase(code: string): string {
  const firstWord = code.trim().split(/\s+/)[0] ?? ''
  const kebab = firstWord
    .replace(/[^A-Za-z0-9-]/g, '')
    .replace(/([a-z])([A-Z])/g, '$1-$2')
    .toLowerCase()
  return kebab && kebab !== 'graph' ? kebab : 'diagram'
}

/** The diagram as a standalone SVG file with an explicit size instead of "100%". */
export function standaloneSvg(diagram: RenderedDiagram): string {
  const end = diagram.svg.indexOf('>')
  const rootTag = diagram.svg
    .slice(0, end)
    .replace(/\swidth="[^"]*"/, '')
    .replace(/\sheight="[^"]*"/, '')
    .replace(/\sstyle="[^"]*"/, '')
  return `${rootTag} width="${diagram.width}" height="${diagram.height}"${diagram.svg.slice(end)}`
}

/** The color behind the diagram in the app, so exports read the same in either theme. */
function surfaceColor(): string {
  const channels = getComputedStyle(document.documentElement).getPropertyValue('--surface').trim()
  return channels ? `hsl(${channels})` : 'white'
}

/** Rasterizes an export render of the diagram to PNG on the current theme's surface color. */
function diagramToPng(diagram: RenderedDiagram): Promise<Blob> {
  if (diagram.svg.includes('<foreignObject')) {
    return Promise.reject(new Error('this diagram type can only be exported as SVG'))
  }
  const svg = standaloneSvg(diagram)
  const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }))
  return new Promise<Blob>((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      const canvas = document.createElement('canvas')
      canvas.width = Math.ceil(diagram.width * PNG_SCALE)
      canvas.height = Math.ceil(diagram.height * PNG_SCALE)
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        reject(new Error('canvas is not available'))
        return
      }
      ctx.fillStyle = surfaceColor()
      ctx.fillRect(0, 0, canvas.width, canvas.height)
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
      try {
        canvas.toBlob(blob => (blob ? resolve(blob) : reject(new Error('PNG export failed'))), 'image/png')
      } catch (err) {
        reject(err instanceof Error ? err : new Error(String(err)))
      }
    }
    img.onerror = () => reject(new Error('the diagram could not be drawn as an image'))
    img.src = url
  }).finally(() => URL.revokeObjectURL(url))
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result).slice(String(reader.result).indexOf(',') + 1))
    reader.onerror = () => reject(reader.error ?? new Error('could not read the image'))
    reader.readAsDataURL(blob)
  })
}

/** The diagram drawn for files: SVG text labels, so it exports to PNG and opens in any editor. */
function exportRender(code: string, theme: Theme): Promise<RenderedDiagram> {
  return renderMermaid(code, theme, { forExport: true })
}

/** Saves the diagram as SVG, PNG or Mermaid source through the native save dialog. */
export function saveDiagram(code: string, theme: Theme, format: DiagramFormat): Promise<void> {
  const name = `${diagramFileBase(code)}.${format}`
  if (format === 'mmd') return saveWithFeedback(fileService.saveFile(name, code), 'Diagram source')
  if (format === 'svg') {
    return saveWithFeedback(
      exportRender(code, theme).then(diagram => fileService.saveFile(name, standaloneSvg(diagram))),
      'Diagram',
    )
  }
  return saveWithFeedback(
    exportRender(code, theme)
      .then(diagramToPng)
      .then(blobToBase64)
      .then(base64 => fileService.saveBase64File(name, base64)),
    'Diagram',
  )
}

/** Copies the diagram as a PNG image. Call it straight from a click handler. */
export function copyDiagramImage(code: string, theme: Theme): Promise<void> {
  return copyPngToClipboard(exportRender(code, theme).then(diagramToPng)).then(
    () => {
      toast.success('Diagram copied as image')
    },
    err => {
      toast.error('Could not copy diagram', { description: err instanceof Error ? err.message : String(err) })
    },
  )
}
