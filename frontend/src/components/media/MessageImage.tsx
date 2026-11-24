import type { ImgHTMLAttributes } from 'react'
import { Copy, Download, Maximize2 } from 'lucide-react'
import { Button } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { isGeneratedImageUrl, isLocalImageSrc } from '../../utils/fileKinds'
import { lightbox } from '../ImageLightbox'
import { copyImage, downloadImage } from './mediaActions'

type MessageImageProps = ImgHTMLAttributes<HTMLImageElement> & { node?: unknown }

/**
 * An image inside a message. Images Pedro holds (generated, pasted, inline) can be
 * opened full size, copied and downloaded; remote images render as plain images.
 */
export function MessageImage({ src, alt, className, node: _node, ...props }: MessageImageProps) {
  const local = isLocalImageSrc(src)
  const generated = isGeneratedImageUrl(src)
  const classes = ['message-image', generated && 'message-image--generated', className].filter(Boolean).join(' ')

  if (!src || !local) {
    return <img src={src} alt={alt ?? 'Image'} className={classes} {...props} />
  }

  return (
    <span className="message-image-frame" data-generated={generated ? '' : undefined}>
      <img
        src={src}
        alt={alt ?? 'Image'}
        className={classes}
        onClick={() => void lightbox.open({ payload: src })}
        {...props}
      />
      <span className="message-image-actions" role="group" aria-label="Image actions">
        <Hint label="View full size">
          <Button variant="overlay" size="icon-xs" onClick={() => void lightbox.open({ payload: src })}>
            <Maximize2 aria-hidden="true" />
          </Button>
        </Hint>
        <Hint label="Copy image">
          <Button variant="overlay" size="icon-xs" onClick={() => void copyImage(src)}>
            <Copy aria-hidden="true" />
          </Button>
        </Hint>
        <Hint label="Download image">
          <Button variant="overlay" size="icon-xs" onClick={() => void downloadImage(src)}>
            <Download aria-hidden="true" />
          </Button>
        </Hint>
      </span>
    </span>
  )
}
