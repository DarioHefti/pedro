import { useState, useEffect, useCallback } from 'react'
import { updaterService, eventService } from '../services/wailsService'
import type { UpdateInfo } from '../services/wailsService'

type UpdateState = 'idle' | 'available' | 'downloading' | 'installing' | 'done' | 'error'

export default function UpdateNotification() {
  const [state, setState] = useState<UpdateState>('idle')
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [dismissed, setDismissed] = useState(false)

  useEffect(() => {
    const timer = setTimeout(async () => {
      try {
        const info = await updaterService.checkForUpdate()
        if (info.available) {
          setUpdateInfo(info)
          setState('available')
        }
      } catch {
        // Silently fail — update check is non-critical
      }
    }, 3000)

    return () => clearTimeout(timer)
  }, [])

  useEffect(() => {
    const cancel = eventService.on('update_progress', (_percent: number, status: string) => {
      if (status === 'downloading') setState('downloading')
      else if (status === 'installing') setState('installing')
      else if (status === 'done') setState('done')
      else if (status === 'error') setState('error')
    })
    return () => { cancel() }
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

  if (dismissed || state === 'idle') return null

  return (
    <div className="update-notification">
      {state === 'available' && (
        <>
          <span className="update-notification-text">
            Update available: <strong>v{updateInfo?.latestVersion}</strong>
          </span>
          <div className="update-notification-actions">
            <button className="update-btn-primary" onClick={handleDownload}>
              Update
            </button>
            <button className="update-btn-dismiss" onClick={() => setDismissed(true)}>
              Later
            </button>
          </div>
        </>
      )}
      {state === 'downloading' && (
        <span className="update-notification-text">
          <span className="update-spinner" /> Downloading update…
        </span>
      )}
      {state === 'installing' && (
        <span className="update-notification-text">
          <span className="update-spinner" /> Installing… The app will restart.
        </span>
      )}
      {state === 'done' && (
        <span className="update-notification-text update-success">
          Update installed! Please restart Pedro.
        </span>
      )}
      {state === 'error' && (
        <>
          <span className="update-notification-text update-error">
            Update failed.
          </span>
          <button className="update-btn-dismiss" onClick={() => setDismissed(true)}>
            Dismiss
          </button>
        </>
      )}
    </div>
  )
}
