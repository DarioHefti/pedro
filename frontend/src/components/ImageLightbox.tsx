import { Copy, Download, X } from 'lucide-react'
import { Dialog, DialogBackdrop, DialogClose, DialogContent, DialogTitle, createDialogController } from '@ui-sdk/react/dialog'
import { Button, buttonStyle } from '../ui/button'
import { Hint } from '../ui/hint'
import { isLocalImageSrc } from '../utils/fileKinds'
import { copyImage, downloadImage } from './media/mediaActions'

/** Opens any image full size: `lightbox.open({ payload: src })`. Mount <ImageLightbox /> once. */
export const lightbox = createDialogController<string>()

function LightboxToolbar({ src }: { src: string }) {
  return (
    <div className="lightbox-toolbar" role="toolbar" aria-label="Image">
      {isLocalImageSrc(src) ? (
        <>
          <Hint label="Copy image" side="bottom">
            <Button variant="overlay" size="icon" onClick={() => void copyImage(src)}>
              <Copy aria-hidden="true" />
            </Button>
          </Hint>
          <Hint label="Download image" side="bottom">
            <Button variant="overlay" size="icon" onClick={() => void downloadImage(src)}>
              <Download aria-hidden="true" />
            </Button>
          </Hint>
        </>
      ) : null}
      <Hint label="Close" shortcut="escape" side="bottom">
        <DialogClose {...buttonStyle({ variant: 'overlay', size: 'icon' })} aria-label="Close preview">
          <X aria-hidden="true" />
        </DialogClose>
      </Hint>
    </div>
  )
}

export function ImageLightbox() {
  return (
    <Dialog controller={lightbox}>
      <DialogBackdrop className="lightbox-backdrop" />
      <DialogContent<string> data-size="media" className="lightbox" initialFocus="content">
        {state => (
          <>
            <DialogTitle className="visually-hidden">Image preview</DialogTitle>
            {state.payload ? (
              <>
                <img src={state.payload} alt="Full size preview" className="lightbox-image" />
                <LightboxToolbar src={state.payload} />
              </>
            ) : null}
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
