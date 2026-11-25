import { useState, useEffect, useCallback } from 'react'
import { CircleAlert, CircleCheck, Copy, Check, Download, X } from 'lucide-react'
import { useCopyToClipboard } from '../hooks/useCopyToClipboard'
import { updaterService, eventService } from '../services/wailsService'
import type { UpdateInfo } from '../services/wailsService'
import { Button } from '../ui/button'
import { Hint } from '../ui/hint'

type UpdateState = 'idle' | 'available' | 'downloading' | 'installing' | 'done' | 'error'

export default function UpdateNotification() {
  const [state, setState] = useState<UpdateState>('idle')
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [dismissed, setDismissed] = useState(false)
  const [defenderExclusion, setDefenderExclusion] = useState('')
  const { copied, copy } = useCopyToClipboard(2000)

  useEffect(() => {
    if (import.meta.env.DEV) return
    const timer = setTimeout(async () => {
      try {
        const info = await updaterService.checkForUpdate()
        if (info.available) {
          setUpdateInfo(info)
          setState('available')
        }
      } catch {
        /* the update check is best effort */
      }
    }, 3000)
    return () => clearTimeout(timer)
  }, [])

  useEffect(() => {
    if (state !== 'available') return
    updaterService.getWindowsDefenderExclusion().then(cmd => {
      if (cmd) setDefenderExclusion(cmd)
    })
  }, [state])

  useEffect(() => {
    const cancel = eventService.on('update_progress', (_percent: number, status: string) => {
      if (status === 'downloading') setState('downloading')
      else if (status === 'installing') setState('installing')
      else if (status === 'done') setState('done')
      else if (status === 'error') setState('error')
    })
    return () => {
      cancel()
    }
  }, [])

  const handleDownload = useCallback(async () => {
    if (!updateInfo) return
    setState('downloading')
    try {
      await updaterService.downloadAndInstall(updateInfo.assetURL, updateInfo.assetName)
    } catch {
      setState('error')
    }
  }, [updateInfo])

  const handleCopy = useCallback(() => {
    if (defenderExclusion) void copy(defenderExclusion)
  }, [defenderExclusion, copy])

  if (dismissed || state === 'idle') return null

  const working = state === 'downloading' || state === 'installing'

  return (
    <aside className="update-card" data-state={state} role="status" aria-live="polite">
      <div className="update-card-head">
        <span className="update-card-icon" aria-hidden="true">
          {working ? (
            <span className="spinner" />
          ) : state === 'done' ? (
            <CircleCheck />
          ) : state === 'error' ? (
            <CircleAlert />
          ) : (
            <Download />
          )}
        </span>
        <div className="update-card-text">
          <strong>
            {state === 'available'
              ? `Pedro ${updateInfo?.latestVersion ? `v${updateInfo.latestVersion}` : ''} is available`
              : state === 'downloading'
                ? 'Downloading update…'
                : state === 'installing'
                  ? 'Installing update…'
                  : state === 'done'
                    ? 'Update installed'
                    : 'Update failed'}
          </strong>
          <span>
            {state === 'available'
              ? 'Install now, or keep chatting and update later.'
              : state === 'installing'
                ? 'Pedro restarts when it finishes.'
                : state === 'done'
                  ? 'Restart Pedro to use the new version.'
                  : state === 'error'
                    ? 'Try again later, or download it from the releases page.'
                    : 'This takes a moment.'}
          </span>
        </div>
        {!working ? (
          <Hint label="Dismiss">
            <Button variant="ghost" size="icon-xs" onClick={() => setDismissed(true)}>
              <X aria-hidden="true" />
            </Button>
          </Hint>
        ) : null}
      </div>

      {working ? <div className="update-progress" aria-hidden="true" /> : null}

      {state === 'available' ? (
        <>
          {defenderExclusion ? (
            <div className="update-exclusion">
              <span>Run in PowerShell as Admin so Defender doesn't block Pedro:</span>
              <div className="update-exclusion-row">
                <code>{defenderExclusion}</code>
                <Hint label={copied ? 'Copied' : 'Copy command'}>
                  <Button variant="ghost" size="icon-xs" onClick={handleCopy}>
                    {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
                  </Button>
                </Hint>
              </div>
            </div>
          ) : null}
          <div className="update-card-actions">
            <Button variant="ghost" size="sm" onClick={() => setDismissed(true)}>
              Later
            </Button>
            <Button variant="primary" size="sm" onClick={handleDownload}>
              <Download aria-hidden="true" />
              Update now
            </Button>
          </div>
        </>
      ) : null}
    </aside>
  )
}
