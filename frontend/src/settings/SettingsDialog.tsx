import { useEffect, useRef, useState, type RefObject } from 'react'
import { Brain, ChartColumn, CircleCheck, Palette, Plug, ScrollText, X, type LucideIcon } from 'lucide-react'
import {
  Dialog,
  DialogBackdrop,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@ui-sdk/react/dialog'
import { useShortcut } from '@ui-sdk/react/shortcuts'
import { Tabs, TabsIndicator, TabsList, TabsPanel, TabsTab } from '@ui-sdk/react/tabs'
import pedroAvatar from '../assets/images/pedro.svg'
import { Button, buttonStyle } from '../ui/button'
import { confirmAction } from '../ui/confirm'
import { Hint } from '../ui/hint'
import { formatVersion } from '../utils/version'
import { AppearancePanel } from './AppearancePanel'
import { ConnectionPanel } from './ConnectionPanel'
import { InstructionsPanel } from './InstructionsPanel'
import { MemoryPanel } from './MemoryPanel'
import type { SettingsServices } from './settingsModel'
import { UsagePanel } from './UsagePanel'
import { useSettingsDraft } from './useSettingsDraft'

type SectionId = 'connection' | 'instructions' | 'memory' | 'appearance' | 'usage'

interface Section {
  id: SectionId
  label: string
  icon: LucideIcon
  description: string
  /** Saved as you go rather than by the Save button. */
  autosave?: boolean
}

const SECTIONS: Section[] = [
  { id: 'connection', label: 'Connection', icon: Plug, description: 'Choose the model provider Pedro talks to.' },
  { id: 'instructions', label: 'Instructions', icon: ScrollText, description: 'Shape how Pedro responds on every request.' },
  { id: 'memory', label: 'Memory', icon: Brain, description: 'Facts Pedro remembers about you across chats.', autosave: true },
  { id: 'appearance', label: 'Appearance', icon: Palette, description: 'Accent color and text size for both themes.' },
  { id: 'usage', label: 'Usage', icon: ChartColumn, description: 'Lifetime requests and token estimates.' },
]

interface SettingsDialogProps {
  open: boolean
  onClose: () => void
  /** Settings were persisted; app-level values should reload. */
  onSaved: () => void
  services: SettingsServices
  onViewDetails: () => void
}

interface SettingsBodyProps extends Omit<SettingsDialogProps, 'open' | 'onClose'> {
  dirtyRef: { current: boolean }
  layerRef: RefObject<HTMLDivElement | null>
}

function SettingsBody({ services, onSaved, onViewDetails, dirtyRef, layerRef }: SettingsBodyProps) {
  const s = useSettingsDraft(services, onSaved)
  const [tab, setTab] = useState<SectionId>('connection')
  const [version, setVersion] = useState('')
  const section = SECTIONS.find(x => x.id === tab) ?? SECTIONS[0]

  useEffect(() => {
    dirtyRef.current = s.dirty
  }, [s.dirty, dirtyRef])

  useEffect(() => {
    services
      .getCurrentVersion()
      .then(v => setVersion(v))
      .catch(() => setVersion(''))
  }, [services])

  useShortcut('mod+s', () => void s.save(), { layer: layerRef, when: () => s.dirty, description: 'Save settings' })

  return (
    <Tabs<SectionId> className="settings-layout" orientation="vertical" value={tab} onValueChange={v => v && setTab(v)}>
      <aside className="settings-nav">
        <div className="settings-nav-head">
          <span className="avatar" data-size="sm">
            <img src={pedroAvatar} alt="" />
          </span>
          <div>
            <DialogTitle>Settings</DialogTitle>
            <DialogDescription className="visually-hidden">Configure Pedro's provider, behavior and appearance.</DialogDescription>
          </div>
        </div>
        <TabsList aria-label="Settings sections">
          <TabsIndicator />
          {SECTIONS.map(sec => {
            const Icon = sec.icon
            return (
              <TabsTab key={sec.id} value={sec.id}>
                <Icon aria-hidden="true" />
                {sec.label}
              </TabsTab>
            )
          })}
        </TabsList>
        <div className="settings-nav-foot">{formatVersion(version, __PEDRO_SOURCE_VERSION__)}</div>
      </aside>

      <section className="settings-main">
        <header className="settings-main-head">
          <div>
            <h3>{section.label}</h3>
            <p>{section.description}</p>
          </div>
          <Hint label="Close" shortcut="escape">
            <DialogClose {...buttonStyle({ variant: 'ghost', size: 'icon-sm' })}>
              <X aria-hidden="true" />
            </DialogClose>
          </Hint>
        </header>

        <div className="settings-main-body">
          <TabsPanel value="connection">
            <ConnectionPanel s={s} />
          </TabsPanel>
          <TabsPanel value="instructions">
            <InstructionsPanel s={s} />
          </TabsPanel>
          <TabsPanel value="memory">
            <MemoryPanel services={services} />
          </TabsPanel>
          <TabsPanel value="appearance">
            <AppearancePanel s={s} />
          </TabsPanel>
          <TabsPanel value="usage">
            <UsagePanel services={services} onViewDetails={onViewDetails} />
          </TabsPanel>
        </div>

        <footer className="dialog-footer">
          <span className="dialog-footer-start" aria-live="polite">
            {s.dirty ? (
              <>
                <span className="status-dot" data-tone="warning" aria-hidden="true" />
                Unsaved changes
              </>
            ) : section.autosave ? (
              'Changes here save automatically'
            ) : s.hydrated ? (
              <>
                <CircleCheck className="saved-icon" aria-hidden="true" />
                All changes saved
              </>
            ) : null}
          </span>
          <DialogClose {...buttonStyle({ variant: 'secondary', size: 'sm' })}>{s.dirty ? 'Cancel' : 'Close'}</DialogClose>
          <Hint label="Save" shortcut="mod+s">
            <Button variant="primary" size="sm" disabled={!s.dirty} focusableWhenDisabled onClick={() => s.save()}>
              Save changes
            </Button>
          </Hint>
        </footer>
      </section>
    </Tabs>
  )
}

export function SettingsDialog({ open, onClose, ...rest }: SettingsDialogProps) {
  const dirtyRef = useRef(false)
  const layerRef = useRef<HTMLDivElement>(null)

  return (
    <Dialog
      open={open}
      onOpenChange={(next, details) => {
        if (next) return
        if (dirtyRef.current) {
          details.cancel()
          void confirmAction({
            title: 'Discard unsaved changes?',
            description: 'Your edits to these settings will be lost.',
            confirmLabel: 'Discard changes',
            cancelLabel: 'Keep editing',
          }).then(ok => {
            if (!ok) return
            dirtyRef.current = false
            onClose()
          })
          return
        }
        onClose()
      }}
    >
      <DialogBackdrop />
      <DialogContent ref={layerRef} data-size="lg" className="settings-dialog">
        <SettingsBody {...rest} dirtyRef={dirtyRef} layerRef={layerRef} />
      </DialogContent>
    </Dialog>
  )
}
