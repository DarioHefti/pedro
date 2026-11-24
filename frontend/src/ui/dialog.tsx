import type { ComponentProps, ReactNode } from 'react'
import { X } from 'lucide-react'
import {
  Dialog,
  DialogBackdrop,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@ui-sdk/react/dialog'
import { buttonStyle } from './button'
import { Hint } from './hint'

type DialogRootProps = ComponentProps<typeof Dialog>
type DialogContentProps = ComponentProps<typeof DialogContent>

export type DialogSize = 'sm' | 'md' | 'lg' | 'xl' | 'palette' | 'media'

interface AppDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  /** Leading icon tile beside the title. */
  icon?: ReactNode
  size?: DialogSize
  /** Buttons placed before the close button in the header. */
  headerActions?: ReactNode
  footer?: ReactNode
  /** Hide the visual header (title stays for screen readers). */
  bareHeader?: boolean
  dismissible?: boolean
  onDismissAttempt?: DialogRootProps['onDismissAttempt']
  initialFocus?: DialogContentProps['initialFocus']
  className?: string
  children: ReactNode
}

/** The design system's dialog shell: backdrop, animated surface, header, body and footer. */
export function AppDialog({
  open,
  onOpenChange,
  title,
  description,
  icon,
  size = 'md',
  headerActions,
  footer,
  bareHeader,
  dismissible,
  onDismissAttempt,
  initialFocus,
  className,
  children,
}: AppDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={next => onOpenChange(next)}
      dismissible={dismissible}
      onDismissAttempt={onDismissAttempt}
    >
      <DialogBackdrop />
      <DialogContent data-size={size} className={className} initialFocus={initialFocus}>
        {bareHeader ? (
          <DialogTitle className="visually-hidden">{title}</DialogTitle>
        ) : (
          <header className="dialog-header">
            {icon ? <span className="dialog-icon">{icon}</span> : null}
            <div className="dialog-header-text">
              <DialogTitle>{title}</DialogTitle>
              {description ? <DialogDescription>{description}</DialogDescription> : null}
            </div>
            {headerActions}
            <Hint label="Close" shortcut="escape">
              <DialogClose {...buttonStyle({ variant: 'ghost', size: 'icon-sm' })}>
                <X aria-hidden="true" />
              </DialogClose>
            </Hint>
          </header>
        )}
        {children}
        {footer ? <footer className="dialog-footer">{footer}</footer> : null}
      </DialogContent>
    </Dialog>
  )
}
