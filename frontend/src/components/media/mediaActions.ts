import { fileService, type SaveResult } from '../../services/wailsService'
import { toast } from '../../ui/toaster'
import { copyImageToClipboard } from '../../utils/clipboard'

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/** Reports a native save: confirms where it went, stays quiet when the person cancelled. */
export async function saveWithFeedback(save: Promise<SaveResult>, what: string): Promise<void> {
  try {
    const result = await save
    if (!result.cancelled) toast.success(`${what} saved`, { description: result.path || undefined })
  } catch (err) {
    toast.error(`Could not save ${what.toLowerCase()}`, { description: errorMessage(err) })
  }
}

/** Saves an image the chat shows (stored image URL or data URL) through the native save dialog. */
export function downloadImage(src: string): Promise<void> {
  return saveWithFeedback(fileService.saveImage(src), 'Image')
}

/**
 * Copies an image to the clipboard as PNG. Call it straight from a click handler:
 * the clipboard write must start inside the user gesture.
 */
export function copyImage(src: string): Promise<void> {
  return copyImageToClipboard(src).then(
    () => {
      toast.success('Image copied to clipboard')
    },
    err => {
      toast.error('Could not copy image', { description: errorMessage(err) })
    },
  )
}
