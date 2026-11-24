import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  applyDesignPaletteToDocument,
  applyMessageFontSizeToDocument,
  applyUiFontSizeToDocument,
  DEFAULT_MESSAGE_FONT_SIZE_PX,
  DEFAULT_UI_FONT_SIZE_PX,
  getDesignPaletteFromSettings,
} from '../designTheme'
import { toast } from '../ui/toaster'
import {
  connectionCheckFromSettings,
  draftFromSettings,
  extraSettingEntries,
  fingerprint,
  normalizeErrorMessage,
  providerRecord,
  type ConnectionCheck,
  type SettingsDraft,
  type SettingsServices,
} from './settingsModel'

export interface SettingsDraftState {
  hydrated: boolean
  draft: SettingsDraft
  update: (patch: Partial<SettingsDraft>) => void
  defaultSystemPrompt: string
  dirty: boolean
  canTest: boolean
  connection: ConnectionCheck
  authenticated: boolean
  save: () => Promise<void>
  test: () => Promise<void>
  signIn: () => Promise<void>
  signOut: () => Promise<void>
  resetAppearance: () => void
}

/**
 * Owns the editable copy of the persisted settings: hydration, dirty tracking against the last
 * persisted fingerprint, live appearance preview, and the save / test / sign-in flows.
 */
export function useSettingsDraft(services: SettingsServices, onSaved?: () => void): SettingsDraftState {
  const [draft, setDraft] = useState<SettingsDraft>(() => draftFromSettings({}))
  const [hydrated, setHydrated] = useState(false)
  const [persistedFp, setPersistedFp] = useState('')
  const [defaultSystemPrompt, setDefaultSystemPrompt] = useState('')
  const [connection, setConnection] = useState<ConnectionCheck>({ kind: 'idle' })
  const [authenticated, setAuthenticated] = useState(false)
  const servicesRef = useRef(services)
  servicesRef.current = services

  useEffect(() => {
    let cancelled = false
    const svc = servicesRef.current
    Promise.all([svc.getSettings(), svc.isAuthenticated(), svc.getDefaultSystemPrompt()])
      .then(([s, auth, defPrompt]) => {
        if (cancelled) return
        const d = draftFromSettings(s)
        const fp = fingerprint(d)
        setDraft(d)
        setDefaultSystemPrompt(defPrompt)
        setAuthenticated(auth)
        setPersistedFp(fp)
        setConnection(connectionCheckFromSettings(s, fp))
        setHydrated(true)
      })
      .catch(err => toast.error('Could not load settings', { description: String(err) }))
    return () => {
      cancelled = true
    }
  }, [])

  const currentFp = useMemo(() => fingerprint(draft), [draft])
  const dirty = hydrated && currentFp !== persistedFp

  useEffect(() => {
    if (dirty && connection.kind !== 'idle') setConnection({ kind: 'idle' })
  }, [dirty, connection.kind])

  useEffect(() => {
    if (!hydrated) return
    applyDesignPaletteToDocument({ lightBase: draft.designLightBaseColor, darkBase: draft.designDarkBaseColor })
    applyUiFontSizeToDocument(draft.uiFontSizePx)
    applyMessageFontSizeToDocument(draft.messageFontSizePx)
  }, [hydrated, draft.designLightBaseColor, draft.designDarkBaseColor, draft.uiFontSizePx, draft.messageFontSizePx])

  const update = useCallback((patch: Partial<SettingsDraft>) => setDraft(prev => ({ ...prev, ...patch })), [])

  const persist = useCallback(async () => {
    const svc = servicesRef.current
    await svc.saveSettings(providerRecord(draft))
    for (const [key, value] of extraSettingEntries(draft)) {
      await svc.setSetting(key, value)
    }
    setPersistedFp(fingerprint(draft))
  }, [draft])

  const syncConnection = useCallback(async () => {
    const s = await servicesRef.current.getSettings()
    setConnection(connectionCheckFromSettings(s, fingerprint(draft)))
  }, [draft])

  const save = useCallback(async () => {
    try {
      await persist()
      await syncConnection()
      toast.success('Settings saved')
      onSaved?.()
    } catch (err) {
      toast.error('Could not save settings', { description: String(err) })
    }
  }, [persist, syncConnection, onSaved])

  const test = useCallback(async () => {
    try {
      await persist()
      const result = await servicesRef.current.testConnection()
      await syncConnection()
      if (result.startsWith('Error:')) {
        toast.error('Connection failed', { description: normalizeErrorMessage(result) })
      } else {
        setAuthenticated(await servicesRef.current.isAuthenticated())
        toast.success(result || 'Connection successful')
      }
    } catch (err) {
      const msg = normalizeErrorMessage('Error: ' + String(err))
      setConnection({ kind: 'fail', message: msg, checkedAt: Date.now() })
      toast.error('Connection failed', { description: msg })
    }
  }, [persist, syncConnection])

  const signIn = useCallback(async () => {
    try {
      await persist()
      await syncConnection()
      const errMsg = await servicesRef.current.signIn()
      if (errMsg && errMsg.startsWith('Error:')) {
        toast.error('Sign in failed', { description: normalizeErrorMessage(errMsg) })
      } else {
        setAuthenticated(true)
        toast.success('Signed in to Azure')
      }
    } catch (err) {
      toast.error('Sign in failed', { description: String(err) })
    }
  }, [persist, syncConnection])

  const signOut = useCallback(async () => {
    try {
      await persist()
      await syncConnection()
      await servicesRef.current.signOut()
      setAuthenticated(false)
      toast.success('Signed out')
    } catch (err) {
      toast.error('Sign out failed', { description: String(err) })
    }
  }, [persist, syncConnection])

  const resetAppearance = useCallback(() => {
    const palette = getDesignPaletteFromSettings({})
    update({
      designLightBaseColor: palette.lightBase,
      designDarkBaseColor: palette.darkBase,
      uiFontSizePx: DEFAULT_UI_FONT_SIZE_PX,
      messageFontSizePx: DEFAULT_MESSAGE_FONT_SIZE_PX,
    })
  }, [update])

  return {
    hydrated,
    draft,
    update,
    defaultSystemPrompt,
    dirty,
    canTest: hydrated && !dirty,
    connection,
    authenticated,
    save,
    test,
    signIn,
    signOut,
    resetAppearance,
  }
}
