/** Matches markdown that is exactly one image and nothing else: ![alt](src "optional title"). */
const SINGLE_IMAGE_MARKDOWN = /^\s*!\[[^\]]*\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"]*")?\s*\)\s*$/

/** The image URL when `markdown` consists of a single image, otherwise null. */
export function singleImageSrc(markdown: string): string | null {
  return SINGLE_IMAGE_MARKDOWN.exec(markdown)?.[1] ?? null
}

/**
 * Copies a message: the image itself when the message is just one image,
 * otherwise the text. Resolves to what was copied.
 */
export function copyMessageContent(content: string): Promise<'image' | 'text'> {
  const src = singleImageSrc(content)
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
export async function copyImageToClipboard(src: string): Promise<void> {
  if (typeof ClipboardItem === 'undefined' || !navigator.clipboard?.write) {
    throw new Error('copying images is not supported here')
  }
  const png = fetchAsPng(src)
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
