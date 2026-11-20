/** URL prefix under which the backend serves generated images (media.GeneratedImagesRoute). */
export const GENERATED_IMAGES_ROUTE = '/generated-images/'

/** Raster image paths for file-ref attachments (aligned with backend image_file_refs.go). */
export function isRasterImageFilePath(path: string): boolean {
  return /\.(png|jpe?g|gif|webp|bmp|tiff?)$/i.test(path.trim())
}

/** Audio formats accepted by the transcribe_audio tool (aligned with tools/transcribe_audio.go). */
export function isAudioFilePath(path: string): boolean {
  return /\.(mp3|mp4|mpeg|mpga|m4a|wav|webm|ogg|oga|flac)$/i.test(path.trim())
}

/** True for images created by the generate_image tool. */
export function isGeneratedImageUrl(src: string | undefined): boolean {
  return !!src && src.startsWith(GENERATED_IMAGES_ROUTE)
}
