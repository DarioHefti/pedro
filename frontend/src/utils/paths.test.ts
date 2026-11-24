import { describe, expect, it } from 'vitest'
import { isAudioFilePath, isGeneratedImageUrl, isLocalImageSrc, isRasterImageFilePath } from './fileKinds'
import { looksLikeLocalFilesystemPath } from './localPath'

describe('looksLikeLocalFilesystemPath', () => {
  it.each(['/Users/me/notes.md', '/tmp', ' /var/log/app.log ', 'C:\\Users\\me', 'd:/work', '\\\\server\\share'])('accepts %s', p => {
    expect(looksLikeLocalFilesystemPath(p)).toBe(true)
  })

  it.each(['/', '//x', '/a', 'notes.md', 'src/app.go', 'https://example.com/a', 'C:'])('rejects %s', p => {
    expect(looksLikeLocalFilesystemPath(p)).toBe(false)
  })
})

describe('file kinds', () => {
  it('detects raster images and audio by extension, case-insensitively', () => {
    expect(['a.png', 'b.JPG', 'c.jpeg', 'd.webp', 'e.tiff'].every(isRasterImageFilePath)).toBe(true)
    expect(isRasterImageFilePath('logo.svg')).toBe(false)
    expect(['a.mp3', 'b.M4A', 'c.wav', 'd.flac'].every(isAudioFilePath)).toBe(true)
    expect(isAudioFilePath('movie.mkv')).toBe(false)
  })

  it('tells images Pedro holds apart from remote ones', () => {
    expect(isGeneratedImageUrl('/generated-images/cat.png')).toBe(true)
    expect(isGeneratedImageUrl('/pasted-images/cat.png')).toBe(false)
    expect(isLocalImageSrc('/pasted-images/cat.png')).toBe(true)
    expect(isLocalImageSrc('data:image/png;base64,AA')).toBe(true)
    expect(isLocalImageSrc('https://example.com/cat.png')).toBe(false)
    expect(isLocalImageSrc(undefined)).toBe(false)
  })
})
