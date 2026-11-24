import { isLocalImageSrc } from './fileKinds'

/** Markdown images: ![alt](src "optional title"). */
const MARKDOWN_IMAGE = /!\[[^\]]*\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"]*")?\s*\)/g

/** The first image in `markdown` that Pedro can copy (generated, pasted or inline), otherwise null. */
export function copyableImageSrc(markdown: string): string | null {
  for (const match of markdown.matchAll(MARKDOWN_IMAGE)) {
    if (isLocalImageSrc(match[1])) return match[1]
  }
  return null
}

/**
 * Copies a message: its image when it shows one (even alongside text), otherwise the text.
 * Resolves to what was copied.
 */
export function copyMessageContent(content: string): Promise<'image' | 'text'> {
  const src = copyableImageSrc(content)
  if (src) {
    return copyImageToClipboard(src).then(() => 'image' as const)
  }
  return navigator.clipboard.writeText(content).then(() => 'text' as const)
}

/**
 * Copies an image as PNG, the one image type every webview clipboard accepts.
 * Must be called directly from a click handler: clipboard.write is started
 * synchronously with a pending blob, which keeps WebKit's user-gesture
 * requirement satisfied while the image loads.
 */
export function copyImageToClipboard(src: string): Promise<void> {
  return copyPngToClipboard(fetchAsPng(src))
}

/**
 * Copies a PNG that may still be in the making. Like copyImageToClipboard, it must be
 * called straight from a click handler so the clipboard write starts inside the gesture.
 */
export async function copyPngToClipboard(png: Promise<Blob>): Promise<void> {
  if (typeof ClipboardItem === 'undefined' || !navigator.clipboard?.write) {
    png.catch(() => undefined)
    throw new Error('copying images is not supported here')
  }
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })])
  } catch (err) {
    // Webviews that only accept resolved blobs reject the pending one with a TypeError.
    if (!(err instanceof TypeError)) throw err
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': await png })])
  }
}

async function fetchAsPng(src: string): Promise<Blob> {
  const res = await fetch(src)
  if (!res.ok) {
    throw new Error(`image could not be loaded (HTTP ${res.status})`)
  }
  const blob = await res.blob()
  return blob.type === 'image/png' ? blob : convertToPng(blob)
}

async function convertToPng(blob: Blob): Promise<Blob> {
  const bitmap = await createImageBitmap(blob)
  const canvas = document.createElement('canvas')
  canvas.width = bitmap.width
  canvas.height = bitmap.height
  canvas.getContext('2d')?.drawImage(bitmap, 0, 0)
  bitmap.close()
  return new Promise((resolve, reject) =>
    canvas.toBlob(png => (png ? resolve(png) : reject(new Error('image could not be converted to PNG'))), 'image/png'),
  )
}
