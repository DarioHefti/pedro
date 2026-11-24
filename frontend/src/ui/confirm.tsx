import { TriangleAlert } from 'lucide-react'
import {
  AlertDialog,
  AlertDialogBackdrop,
  AlertDialogClose,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  createConfirm,
} from '@ui-sdk/react/alert-dialog'
import { buttonStyle } from './button'

export interface ConfirmRequest {
  title: string
  description?: string
  confirmLabel?: string
  cancelLabel?: string
  /** "danger" for destructive answers (the default). */
  tone?: 'danger' | 'default'
}

/** `await confirmAction({...})` resolves true when the person agrees. Mount <ConfirmHost /> once. */
export const confirmAction = createConfirm<ConfirmRequest>()

export function ConfirmHost() {
  return (
    <AlertDialog controller={confirmAction.controller}>
      <AlertDialogBackdrop />
      <AlertDialogContent<ConfirmRequest>>
        {state => {
          const q = state.payload
          const danger = (q?.tone ?? 'danger') === 'danger'
          return (
            <>
              {danger ? (
                <span className="alert-icon" aria-hidden="true">
                  <TriangleAlert />
                </span>
              ) : null}
              <AlertDialogTitle>{q?.title ?? ''}</AlertDialogTitle>
              {q?.description ? <AlertDialogDescription>{q.description}</AlertDialogDescription> : null}
              <div className="alert-actions">
                <AlertDialogClose {...buttonStyle({ variant: 'secondary' })}>
                  {q?.cancelLabel ?? 'Cancel'}
                </AlertDialogClose>
                <AlertDialogClose
                  value="confirm"
                  data-agent-confirm={q?.description ?? q?.title}
                  {...buttonStyle({ variant: danger ? 'danger' : 'primary' })}
                >
                  {q?.confirmLabel ?? 'Confirm'}
                </AlertDialogClose>
              </div>
            </>
          )
        }}
      </AlertDialogContent>
    </AlertDialog>
  )
}
