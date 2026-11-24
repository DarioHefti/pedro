import { CircleAlert, CircleCheck, Info, LoaderCircle, TriangleAlert, X } from 'lucide-react'
import {
  Toast,
  ToastClose,
  ToastDescription,
  ToastRegion,
  ToastTitle,
} from '@ui-sdk/react/toast'

export { toast } from '@ui-sdk/react/toast'

/** Renders the app's toast stack. Mount once near the root; call `toast()` from anywhere. */
export function Toaster() {
  return (
    <ToastRegion side="bottom" align="end" maxVisible={4}>
      {() => (
        <Toast>
          <span className="toast-icon" aria-hidden="true">
            <CircleCheck data-show="success" />
            <CircleAlert data-show="error" />
            <TriangleAlert data-show="warning" />
            <LoaderCircle data-show="loading" />
            <Info data-show="info" />
          </span>
          <div className="toast-text">
            <ToastTitle />
            <ToastDescription />
          </div>
          <ToastClose aria-label="Dismiss notification">
            <X aria-hidden="true" />
          </ToastClose>
        </Toast>
      )}
    </ToastRegion>
  )
}
