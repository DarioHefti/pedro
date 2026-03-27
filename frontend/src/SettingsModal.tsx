import { useState, useEffect, useMemo, useCallback } from 'react'
import { useToast } from './context/ToastContext'
import {
  applyDesignPaletteToDocument,
  applyMessageFontSizeToDocument,
  applyUiFontSizeToDocument,
  DEFAULT_MESSAGE_FONT_SIZE_PX,
  DEFAULT_UI_FONT_SIZE_PX,
  getDesignPaletteFromSettings,
  getDesignSettingsKeys,
  getMessageFontSizePxFromSettings,
  getUiFontSizePxFromSettings,
  MESSAGE_FONT_SIZE_SLIDER_MAX_PX,
  MESSAGE_FONT_SIZE_SLIDER_MIN_PX,
  normalizeHex,
  UI_FONT_SIZE_SLIDER_MAX_PX,
  UI_FONT_SIZE_SLIDER_MIN_PX,
} from './designTheme'
import type { Persona } from './services/wailsService'

const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'

type Tab = 'llm' | 'prompt' | 'personas' | 'design'

type ConnectionCheckState =
  | { kind: 'idle' }
  | { kind: 'ok'; message: string; checkedAt: number }
  | { kind: 'fail'; message: string; checkedAt: number }

interface FullSettingsSnapshot {
  providerType: string
  endpoint: string
  deployment: string
  azureApiKey: string
  azureTenantId: string
  apiKey: string
  model: string
  systemPrompt: string
  customSystemPrompt: string
  welcomeMessage: string
  designLightBaseColor: string
  designDarkBaseColor: string
  uiFontSizePx: number
  messageFontSizePx: number
}

function buildFullSettingsRecord(s: FullSettingsSnapshot): Record<string, string> {
  const settings: Record<string, string> = {
    provider_type: s.providerType,
    system_prompt: s.systemPrompt,
    custom_system_prompt: s.customSystemPrompt,
    welcome_message: s.welcomeMessage,
    [getDesignSettingsKeys().light]: s.designLightBaseColor,
    [getDesignSettingsKeys().dark]: s.designDarkBaseColor,
    [getDesignSettingsKeys().uiFontSizePx]: String(s.uiFontSizePx),
    [getDesignSettingsKeys().messageFontSizePx]: String(s.messageFontSizePx),
  }

  if (s.providerType === 'azure') {
    settings.azure_endpoint = s.endpoint
    settings.azure_deployment = s.deployment
    settings.azure_tenant_id = s.azureTenantId.trim()
  } else if (s.providerType === 'azure_apikey') {
    settings.azure_endpoint = s.endpoint
    settings.azure_deployment = s.deployment
    settings.azure_api_key = s.azureApiKey
  } else if (s.providerType === 'openai') {
    settings.openai_api_key = s.apiKey
    settings.openai_model = s.model
  }

  return settings
}

function fingerprintFromSnapshot(s: FullSettingsSnapshot): string {
  const rec = buildFullSettingsRecord(s)
  const keys = Object.keys(rec).sort()
  return JSON.stringify(keys.map(k => [k, rec[k]]))
}

function formatCheckedAt(ts: number): string {
  try {
    return new Intl.DateTimeFormat(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }).format(new Date(ts))
  } catch {
    return ''
  }
}

function normalizeErrorMessage(raw: string): string {
  const trimmed = raw.trim()
  if (trimmed.startsWith('Error:')) {
    return trimmed.slice(6).trim() || trimmed
  }
  return trimmed
}

const STORED_CONNECTION_KEY = 'connection_test'

function connectionCheckFromStoredSettings(
  s: Record<string, string>,
  fingerprint: string
): ConnectionCheckState {
  const raw = s[STORED_CONNECTION_KEY]?.trim()
  if (!raw) {
    return { kind: 'idle' }
  }
  try {
    const o = JSON.parse(raw) as {
      ok?: boolean
      message?: string
      at?: number
      fingerprint?: string
    }
    if (typeof o.fingerprint !== 'string' || o.fingerprint !== fingerprint) {
      return { kind: 'idle' }
    }
    const at = typeof o.at === 'number' && !Number.isNaN(o.at) ? o.at : Date.now()
    if (o.ok === true) {
      return {
        kind: 'ok',
        message: typeof o.message === 'string' ? o.message : 'Connection successful!',
        checkedAt: at,
      }
    }
    if (o.ok === false) {
      return {
        kind: 'fail',
        message: typeof o.message === 'string' ? o.message : 'Connection failed',
        checkedAt: at,
      }
    }
  } catch {
    /* ignore */
  }
  return { kind: 'idle' }
}

interface SettingsModalProps {
  onClose: () => void
  onSaved?: () => void
  getSettings: () => Promise<Record<string, string>>
  getDefaultSystemPrompt: () => Promise<string>
  saveSettings: (settings: Record<string, string>) => Promise<void>
  setSetting: (key: string, value: string) => Promise<void>
  testConnection: () => Promise<string>
  signIn: () => Promise<string>
  signOut: () => Promise<void>
  isAuthenticated: () => Promise<boolean>
  getPersonas: () => Promise<Persona[]>
  createPersona: (name: string, prompt: string) => Promise<Persona>
  updatePersona: (id: number, name: string, prompt: string) => Promise<void>
  deletePersona: (id: number) => Promise<void>
}

export default function SettingsModal({
  onClose,
  onSaved,
  getSettings,
  getDefaultSystemPrompt,
  saveSettings,
  setSetting,
  testConnection,
  signIn,
  signOut,
  isAuthenticated,
  getPersonas,
  createPersona,
  updatePersona,
  deletePersona,
}: SettingsModalProps) {
  const toast = useToast()
  const [activeTab, setActiveTab] = useState<Tab>('llm')

  // LLM Settings
  const [providerType, setProviderType] = useState('azure')
  const [endpoint, setEndpoint] = useState('')
  const [deployment, setDeployment] = useState('')
  const [azureApiKey, setAzureApiKey] = useState('')
  const [azureTenantId, setAzureTenantId] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState('gpt-4o')
  const [authenticated, setAuthenticated] = useState(false)
  const [signingIn, setSigningIn] = useState(false)
  const [signingOut, setSigningOut] = useState(false)
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)

  const [hydrated, setHydrated] = useState(false)
  const [lastPersistedFingerprint, setLastPersistedFingerprint] = useState('')
  const [connectionCheck, setConnectionCheck] = useState<ConnectionCheckState>({ kind: 'idle' })

  // Prompt Settings
  const [systemPrompt, setSystemPrompt] = useState('')
  const [defaultSystemPrompt, setDefaultSystemPrompt] = useState('')
  const [customSystemPrompt, setCustomSystemPrompt] = useState('')
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)
  const [designLightBaseColor, setDesignLightBaseColor] = useState(getDesignPaletteFromSettings({}).lightBase)
  const [designDarkBaseColor, setDesignDarkBaseColor] = useState(getDesignPaletteFromSettings({}).darkBase)
  const [persistedDesignLightBaseColor, setPersistedDesignLightBaseColor] = useState(
    getDesignPaletteFromSettings({}).lightBase
  )
  const [persistedDesignDarkBaseColor, setPersistedDesignDarkBaseColor] = useState(
    getDesignPaletteFromSettings({}).darkBase
  )
  const [uiFontSizePx, setUiFontSizePx] = useState(() => getUiFontSizePxFromSettings({}))
  const [persistedUiFontSizePx, setPersistedUiFontSizePx] = useState(() => getUiFontSizePxFromSettings({}))
  const [messageFontSizePx, setMessageFontSizePx] = useState(() =>
    getMessageFontSizePxFromSettings({}),
  )
  const [persistedMessageFontSizePx, setPersistedMessageFontSizePx] = useState(() =>
    getMessageFontSizePxFromSettings({}),
  )
  const designSettingsKeys = getDesignSettingsKeys()

  // Personas (persisted immediately; not part of main Save fingerprint)
  const [personasList, setPersonasList] = useState<Persona[]>([])
  const [personaBusy, setPersonaBusy] = useState(false)
  const [newPersonaName, setNewPersonaName] = useState('')
  const [newPersonaPrompt, setNewPersonaPrompt] = useState('')
  const [editDraft, setEditDraft] = useState<{ id: number; name: string; prompt: string } | null>(
    null,
  )

  const snapshot = useMemo<FullSettingsSnapshot>(
    () => ({
      providerType,
      endpoint,
      deployment,
      azureApiKey,
      azureTenantId,
      apiKey,
      model,
      systemPrompt,
      customSystemPrompt,
      welcomeMessage,
      designLightBaseColor,
      designDarkBaseColor,
      uiFontSizePx,
      messageFontSizePx,
    }),
    [
      providerType,
      endpoint,
      deployment,
      azureApiKey,
      azureTenantId,
      apiKey,
      model,
      systemPrompt,
      customSystemPrompt,
      welcomeMessage,
      designLightBaseColor,
      designDarkBaseColor,
      uiFontSizePx,
      messageFontSizePx,
    ]
  )

  const currentFingerprint = useMemo(() => fingerprintFromSnapshot(snapshot), [snapshot])

  const hasUnsavedChanges = hydrated && currentFingerprint !== lastPersistedFingerprint
  const canTestConnection = hydrated && !hasUnsavedChanges

  useEffect(() => {
    Promise.all([getSettings(), isAuthenticated(), getDefaultSystemPrompt()]).then(([s, auth, defPrompt]) => {
      const pt = s.provider_type ?? 'azure'
      const ep = s.azure_endpoint ?? ''
      const dep = s.azure_deployment ?? ''
      const ak = s.azure_api_key ?? ''
      const tid = s.azure_tenant_id ?? ''
      const ok = s.openai_api_key ?? ''
      const m = s.openai_model ?? 'gpt-4o'
      const sp = s.system_prompt ?? ''
      const csp = s.custom_system_prompt ?? ''
      const wm = s.welcome_message ?? DEFAULT_WELCOME_MESSAGE
      setDefaultSystemPrompt(defPrompt)
      const designPalette = getDesignPaletteFromSettings(s)
      const uiPx = getUiFontSizePxFromSettings(s)
      const msgPx = getMessageFontSizePxFromSettings(s)

      setProviderType(pt)
      setEndpoint(ep)
      setDeployment(dep)
      setAzureApiKey(ak)
      setAzureTenantId(tid)
      setApiKey(ok)
      setModel(m)
      setAuthenticated(auth)
      setSystemPrompt(sp)
      setCustomSystemPrompt(csp)
      setWelcomeMessage(wm)
      setDesignLightBaseColor(designPalette.lightBase)
      setDesignDarkBaseColor(designPalette.darkBase)
      setPersistedDesignLightBaseColor(designPalette.lightBase)
      setPersistedDesignDarkBaseColor(designPalette.darkBase)
      setUiFontSizePx(uiPx)
      setPersistedUiFontSizePx(uiPx)
      setMessageFontSizePx(msgPx)
      setPersistedMessageFontSizePx(msgPx)

      const fp = fingerprintFromSnapshot({
        providerType: pt,
        endpoint: ep,
        deployment: dep,
        azureApiKey: ak,
        azureTenantId: tid,
        apiKey: ok,
        model: m,
        systemPrompt: sp,
        customSystemPrompt: csp,
        welcomeMessage: wm,
        designLightBaseColor: designPalette.lightBase,
        designDarkBaseColor: designPalette.darkBase,
        uiFontSizePx: uiPx,
        messageFontSizePx: msgPx,
      })
      setLastPersistedFingerprint(fp)
      setConnectionCheck(connectionCheckFromStoredSettings(s, fp))
      setHydrated(true)
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (hasUnsavedChanges && connectionCheck.kind !== 'idle') {
      setConnectionCheck({ kind: 'idle' })
    }
  }, [hasUnsavedChanges, connectionCheck.kind])

  useEffect(() => {
    applyDesignPaletteToDocument({
      lightBase: designLightBaseColor,
      darkBase: designDarkBaseColor,
    })
    applyUiFontSizeToDocument(uiFontSizePx)
    applyMessageFontSizeToDocument(messageFontSizePx)
  }, [designLightBaseColor, designDarkBaseColor, uiFontSizePx, messageFontSizePx])

  function buildProviderSettings(): Record<string, string> {
    const { providerType: pt, endpoint: ep, deployment: dep, azureApiKey: aak, azureTenantId: tid, apiKey: ok, model: mo } =
      snapshot
    const settings: Record<string, string> = {
      provider_type: pt,
    }

    if (pt === 'azure') {
      settings.azure_endpoint = ep
      settings.azure_deployment = dep
      settings.azure_tenant_id = tid.trim()
    } else if (pt === 'azure_apikey') {
      settings.azure_endpoint = ep
      settings.azure_deployment = dep
      settings.azure_api_key = aak
    } else if (pt === 'openai') {
      settings.openai_api_key = ok
      settings.openai_model = mo
    }

    return settings
  }

  async function applyProviderSettings() {
    await saveSettings(buildProviderSettings())
    await setSetting('system_prompt', systemPrompt)
    await setSetting('custom_system_prompt', customSystemPrompt)
    await setSetting('welcome_message', welcomeMessage)
    await setSetting(designSettingsKeys.light, designLightBaseColor)
    await setSetting(designSettingsKeys.dark, designDarkBaseColor)
    await setSetting(designSettingsKeys.uiFontSizePx, String(uiFontSizePx))
    await setSetting(designSettingsKeys.messageFontSizePx, String(messageFontSizePx))
  }

  function markPersistedFromState() {
    setLastPersistedFingerprint(fingerprintFromSnapshot(snapshot))
    setPersistedDesignLightBaseColor(designLightBaseColor)
    setPersistedDesignDarkBaseColor(designDarkBaseColor)
    setPersistedUiFontSizePx(uiFontSizePx)
    setPersistedMessageFontSizePx(messageFontSizePx)
  }

  function handleClose() {
    applyDesignPaletteToDocument({
      lightBase: persistedDesignLightBaseColor,
      darkBase: persistedDesignDarkBaseColor,
    })
    applyUiFontSizeToDocument(persistedUiFontSizePx)
    applyMessageFontSizeToDocument(persistedMessageFontSizePx)
    onClose()
  }

  function updateDesignLightColor(rawValue: string) {
    setDesignLightBaseColor(prev => normalizeHex(rawValue, prev))
  }

  function updateDesignDarkColor(rawValue: string) {
    setDesignDarkBaseColor(prev => normalizeHex(rawValue, prev))
  }

  function resetDesignColors() {
    const defaults = getDesignPaletteFromSettings({})
    setDesignLightBaseColor(defaults.lightBase)
    setDesignDarkBaseColor(defaults.darkBase)
    setUiFontSizePx(DEFAULT_UI_FONT_SIZE_PX)
    setMessageFontSizePx(DEFAULT_MESSAGE_FONT_SIZE_PX)
  }

  async function syncConnectionCheckFromBackend() {
    const s = await getSettings()
    setConnectionCheck(connectionCheckFromStoredSettings(s, fingerprintFromSnapshot(snapshot)))
  }

  async function save() {
    setSaving(true)
    try {
      await applyProviderSettings()
      markPersistedFromState()
      await syncConnectionCheckFromBackend()
      toast.success('Settings saved!')
      onSaved?.()
    } catch (err) {
      toast.error('Save error: ' + String(err))
    } finally {
      setSaving(false)
    }
  }

  async function handleSignIn() {
    setSigningIn(true)
    try {
      await applyProviderSettings()
      markPersistedFromState()
      await syncConnectionCheckFromBackend()
      const errMsg = await signIn()
      if (errMsg && errMsg.startsWith('Error:')) {
        toast.error(errMsg)
      } else {
        setAuthenticated(true)
        toast.success('Signed in successfully!')
      }
    } catch (err) {
      toast.error('Sign in error: ' + String(err))
    } finally {
      setSigningIn(false)
    }
  }

  async function handleSignOut() {
    setSigningOut(true)
    try {
      await applyProviderSettings()
      markPersistedFromState()
      await syncConnectionCheckFromBackend()
      await signOut()
      setAuthenticated(false)
      toast.success('Signed out')
    } catch (err) {
      toast.error('Sign out error: ' + String(err))
    } finally {
      setSigningOut(false)
    }
  }

  async function test() {
    setTesting(true)
    try {
      await applyProviderSettings()
      markPersistedFromState()
      const result = await testConnection()
      await syncConnectionCheckFromBackend()
      if (result.startsWith('Error:')) {
        toast.error(result)
      } else {
        const auth = await isAuthenticated()
        setAuthenticated(auth)
        toast.success(result)
      }
    } catch (err) {
      const raw = 'Error: ' + String(err)
      const msg = normalizeErrorMessage(raw)
      setConnectionCheck({ kind: 'fail', message: msg, checkedAt: Date.now() })
      toast.error(raw)
    } finally {
      setTesting(false)
    }
  }

  const loadPersonas = useCallback(async () => {
    try {
      const list = await getPersonas()
      setPersonasList(list)
    } catch (e) {
      toast.error('Failed to load personas: ' + String(e))
    }
  }, [getPersonas])

  useEffect(() => {
    if (activeTab !== 'personas') return
    void loadPersonas()
  }, [activeTab, loadPersonas])

  async function handleAddPersona() {
    const name = newPersonaName.trim()
    if (!name) {
      toast.error('Persona name is required')
      return
    }
    setPersonaBusy(true)
    try {
      await createPersona(name, newPersonaPrompt)
      setNewPersonaName('')
      setNewPersonaPrompt('')
      await loadPersonas()
      onSaved?.()
    } catch (e) {
      toast.error(String(e))
    } finally {
      setPersonaBusy(false)
    }
  }

  async function handleSaveEditPersona() {
    if (!editDraft) return
    const name = editDraft.name.trim()
    if (!name) {
      toast.error('Persona name is required')
      return
    }
    setPersonaBusy(true)
    try {
      await updatePersona(editDraft.id, name, editDraft.prompt)
      setEditDraft(null)
      await loadPersonas()
      onSaved?.()
    } catch (e) {
      toast.error(String(e))
    } finally {
      setPersonaBusy(false)
    }
  }

  async function handleDeletePersona(id: number) {
    if (!window.confirm('Delete this persona?')) return
    setPersonaBusy(true)
    try {
      await deletePersona(id)
      if (editDraft?.id === id) setEditDraft(null)
      await loadPersonas()
      onSaved?.()
    } catch (e) {
      toast.error(String(e))
    } finally {
      setPersonaBusy(false)
    }
  }

  const connectionBadgeDotClass =
    connectionCheck.kind === 'ok'
      ? 'connection-dot connection-dot--ok'
      : connectionCheck.kind === 'fail'
        ? 'connection-dot connection-dot--fail'
        : 'connection-dot connection-dot--idle'

  const connectionBadgeLabel =
    !hydrated
      ? 'Loading…'
      : connectionCheck.kind === 'ok'
        ? 'Connected'
        : connectionCheck.kind === 'fail'
          ? 'Failed'
          : 'Not tested'

  const connectionBadgeTitle =
    !hydrated
      ? ''
      : connectionCheck.kind === 'ok'
        ? `${connectionCheck.message} (${formatCheckedAt(connectionCheck.checkedAt)})`
        : connectionCheck.kind === 'fail'
          ? connectionCheck.message
          : hasUnsavedChanges
            ? 'Save settings, then run Test connection'
            : 'Run Test connection to verify'

  return (
    <div className="modal" onClick={handleClose}>
      <div className="modal-content settings-modal" onClick={e => e.stopPropagation()}>
        <h2>Settings</h2>

        {/* Tab Navigation */}
        <div className="settings-tabs">
          <button
            className={`settings-tab ${activeTab === 'llm' ? 'active' : ''}`}
            onClick={() => setActiveTab('llm')}
          >
            LLM
          </button>
          <button
            className={`settings-tab ${activeTab === 'prompt' ? 'active' : ''}`}
            onClick={() => setActiveTab('prompt')}
          >
            Prompt
          </button>
          <button
            className={`settings-tab ${activeTab === 'personas' ? 'active' : ''}`}
            onClick={() => setActiveTab('personas')}
          >
            Personas
          </button>
          <button
            className={`settings-tab ${activeTab === 'design' ? 'active' : ''}`}
            onClick={() => setActiveTab('design')}
          >
            Design
          </button>
        </div>

        {/* Tab Content */}
        <div className="settings-tab-content">
          {activeTab === 'llm' && (
            <div className="settings-panel">
              <div className="settings-provider-block">
                <label htmlFor="settings-provider-select">Provider</label>
                <div className="settings-provider-row">
                  <select
                    id="settings-provider-select"
                    value={providerType}
                    onChange={e => setProviderType(e.target.value)}
                    className="provider-select settings-provider-select"
                  >
                    <option value="azure">Azure OpenAI (Login)</option>
                    <option value="azure_apikey">Azure OpenAI (API Key)</option>
                    <option value="openai">OpenAI</option>
                  </select>
                  <div
                    className="settings-connection-badge"
                    title={connectionBadgeTitle}
                  >
                    <span className={connectionBadgeDotClass} aria-hidden />
                    <span className="settings-connection-badge-label">{connectionBadgeLabel}</span>
                  </div>
                </div>
              </div>

              {connectionCheck.kind === 'fail' && (
                <p className="settings-connection-error" role="status">
                  {connectionCheck.message}
                </p>
              )}

              {hasUnsavedChanges && hydrated && (
                <p className="settings-save-hint" role="status">
                  Save your settings to enable Test connection.
                </p>
              )}

              {(providerType === 'azure' || providerType === 'azure_apikey') && (
                <>
                  <label>Azure OpenAI resource URL</label>
                  <p className="settings-description">
                    API base URL for your OpenAI resource (e.g. https://your-resource.openai.azure.com).
                  </p>
                  <input
                    type="text"
                    value={endpoint}
                    onChange={e => setEndpoint(e.target.value)}
                    placeholder="https://your-resource.openai.azure.com"
                  />
                  <label>Deployment Name</label>
                  <input
                    type="text"
                    value={deployment}
                    onChange={e => setDeployment(e.target.value)}
                    placeholder="gpt-4"
                  />
                </>
              )}

              {providerType === 'azure' && (
                <>
                  <label>Directory (tenant) ID — optional</label>
                  <p className="settings-description">
                    Use your Azure AD tenant GUID or domain if your org requires a specific tenant for sign-in.
                    Leave empty for the default multi-tenant sign-in experience.
                  </p>
                  <input
                    type="text"
                    value={azureTenantId}
                    onChange={e => setAzureTenantId(e.target.value)}
                    placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx or contoso.onmicrosoft.com"
                  />
                </>
              )}

              {providerType === 'azure_apikey' && (
                <>
                  <label>API Key</label>
                  <input
                    type="password"
                    value={azureApiKey}
                    onChange={e => setAzureApiKey(e.target.value)}
                    placeholder="Enter your Azure OpenAI API key"
                  />
                </>
              )}

              {providerType === 'openai' && (
                <>
                  <label>API Key</label>
                  <input
                    type="password"
                    value={apiKey}
                    onChange={e => setApiKey(e.target.value)}
                    placeholder="sk-..."
                  />
                  <label>Model</label>
                  <input
                    type="text"
                    value={model}
                    onChange={e => setModel(e.target.value)}
                    placeholder="gpt-4o"
                  />
                </>
              )}

              {/* Auth status */}
              <div className="auth-status">
                <span className={`auth-dot ${authenticated ? 'auth-dot--signed-in' : 'auth-dot--signed-out'}`} />
                <span className="auth-status-text">
                  {authenticated ? 'Signed in' : 'Not signed in'}
                </span>
              </div>

              <div className="settings-test-row">
                {providerType === 'azure' && (
                  authenticated ? (
                    <button
                      className="sign-out-btn"
                      onClick={handleSignOut}
                      disabled={signingOut}
                    >
                      {signingOut ? 'Signing out...' : 'Sign Out'}
                    </button>
                  ) : (
                    <button
                      className="sign-in-btn"
                      onClick={handleSignIn}
                      disabled={signingIn}
                    >
                      {signingIn ? 'Opening browser...' : 'Sign In with Azure'}
                    </button>
                  )
                )}
                <button
                  className="test-btn"
                  onClick={test}
                  disabled={testing || !canTestConnection}
                  title={!canTestConnection && hydrated ? 'Save your settings to enable connection test.' : undefined}
                >
                  {testing ? 'Testing...' : 'Test Connection'}
                </button>
              </div>
            </div>
          )}

          {activeTab === 'prompt' && (
            <div className="settings-panel">
              <label>Additional Instructions</label>
              <p className="settings-description">
                Extra rules appended after the system prompt (e.g. tone, preferred sources).
              </p>
              <textarea
                className="system-prompt-textarea"
                value={customSystemPrompt}
                onChange={e => setCustomSystemPrompt(e.target.value)}
                placeholder="Enter your custom instructions here...&#10;&#10;Examples:&#10;- Always respond in a formal tone&#10;- Prioritize Swiss sources when available&#10;- Use bullet points for lists"
                rows={6}
              />

              <details className="settings-collapsible">
                <summary className="settings-collapsible-summary">System Prompt</summary>
                <div className="settings-collapsible-body">
                  <div className="settings-label-row">
                    <p className="settings-description" style={{ margin: 0 }}>
                      The base instructions that define the assistant's identity and behavior.
                    </p>
                    <button
                      type="button"
                      className="settings-inline-btn"
                      onClick={() => setSystemPrompt(defaultSystemPrompt)}
                      disabled={systemPrompt === defaultSystemPrompt || (!systemPrompt && !defaultSystemPrompt)}
                      title="Restore the built-in default system prompt"
                    >
                      Reset to default
                    </button>
                  </div>
                  <textarea
                    className="system-prompt-textarea system-prompt-textarea--large"
                    value={systemPrompt || defaultSystemPrompt}
                    onChange={e => setSystemPrompt(e.target.value)}
                    rows={14}
                  />
                </div>
              </details>
            </div>
          )}

          {activeTab === 'personas' && (
            <div className="settings-panel settings-panel--personas">
              <p className="settings-description">
                Personas add instruction text at the top of your message when you send. With two or more
                personas, you can choose which one to use next to the attachment button in the chat.
              </p>

              <div className="persona-add-block">
                <label htmlFor="new-persona-name">Name</label>
                <input
                  id="new-persona-name"
                  type="text"
                  value={newPersonaName}
                  onChange={e => setNewPersonaName(e.target.value)}
                  placeholder="e.g. Software architect"
                  disabled={personaBusy}
                />
                <label htmlFor="new-persona-prompt">Instruction text</label>
                <p className="settings-description">
                  This text is prepended to your outgoing message (not shown in the chat bubble).
                </p>
                <textarea
                  id="new-persona-prompt"
                  className="persona-prompt-textarea"
                  value={newPersonaPrompt}
                  onChange={e => setNewPersonaPrompt(e.target.value)}
                  placeholder="You are an expert software architect. Prefer clear tradeoffs and diagrams in prose."
                  rows={4}
                  disabled={personaBusy}
                />
                <button
                  type="button"
                  className="persona-add-btn"
                  onClick={() => void handleAddPersona()}
                  disabled={personaBusy}
                >
                  {personaBusy ? 'Saving…' : 'Add persona'}
                </button>
              </div>

              <div className="persona-list-header">Saved personas</div>
              {personasList.length === 0 ? (
                <p className="settings-description persona-list-empty">No personas yet.</p>
              ) : (
                <ul className="persona-list">
                  {personasList.map(p => {
                    const isEditing = editDraft?.id === p.ID
                    return (
                      <li key={p.ID} className="persona-list-item">
                        {isEditing && editDraft ? (
                          <div className="persona-edit-form">
                            <label>Name</label>
                            <input
                              type="text"
                              value={editDraft.name}
                              onChange={e =>
                                setEditDraft({ ...editDraft, name: e.target.value })
                              }
                              disabled={personaBusy}
                            />
                            <label>Instruction text</label>
                            <textarea
                              className="persona-prompt-textarea"
                              value={editDraft.prompt}
                              onChange={e =>
                                setEditDraft({ ...editDraft, prompt: e.target.value })
                              }
                              rows={4}
                              disabled={personaBusy}
                            />
                            <div className="persona-edit-actions">
                              <button
                                type="button"
                                className="persona-save-btn"
                                onClick={() => void handleSaveEditPersona()}
                                disabled={personaBusy}
                              >
                                Save
                              </button>
                              <button
                                type="button"
                                className="persona-cancel-btn"
                                onClick={() => setEditDraft(null)}
                                disabled={personaBusy}
                              >
                                Cancel
                              </button>
                            </div>
                          </div>
                        ) : (
                          <div className="persona-row-summary">
                            <div className="persona-row-text">
                              <span className="persona-row-name">{p.Name}</span>
                              <span className="persona-row-preview" title={p.Prompt}>
                                {p.Prompt.length > 120 ? `${p.Prompt.slice(0, 120)}…` : p.Prompt}
                              </span>
                            </div>
                            <div className="persona-row-actions">
                              <button
                                type="button"
                                className="persona-edit-link"
                                onClick={() =>
                                  setEditDraft({ id: p.ID, name: p.Name, prompt: p.Prompt })
                                }
                                disabled={personaBusy}
                              >
                                Edit
                              </button>
                              <button
                                type="button"
                                className="persona-delete-link"
                                onClick={() => void handleDeletePersona(p.ID)}
                                disabled={personaBusy}
                              >
                                Delete
                              </button>
                            </div>
                          </div>
                        )}
                      </li>
                    )
                  })}
                </ul>
              )}
            </div>
          )}

          {activeTab === 'design' && (
            <div className="settings-panel">
              <label className="welcome-message-label">Welcome Message</label>
              <input
                type="text"
                value={welcomeMessage}
                onChange={e => setWelcomeMessage(e.target.value)}
                placeholder={DEFAULT_WELCOME_MESSAGE}
              />

              <div className="design-color-row">
                <label htmlFor="light-design-color">Light theme base color</label>
                <div className="design-color-control-row">
                  <input
                    id="light-design-color"
                    className="design-color-picker"
                    type="color"
                    value={designLightBaseColor}
                    onChange={e => updateDesignLightColor(e.target.value)}
                  />
                  <span className="design-color-hex">{designLightBaseColor}</span>
                </div>
              </div>

              <div className="design-color-row">
                <label htmlFor="dark-design-color">Dark theme base color</label>
                <div className="design-color-control-row">
                  <input
                    id="dark-design-color"
                    className="design-color-picker"
                    type="color"
                    value={designDarkBaseColor}
                    onChange={e => updateDesignDarkColor(e.target.value)}
                  />
                  <span className="design-color-hex">{designDarkBaseColor}</span>
                </div>
              </div>

              <div className="design-preview-grid">
                <div className="design-preview-tile">
                  <span className="design-preview-label">Light preview</span>
                  <span className="design-preview-swatch design-preview-swatch--light" />
                </div>
                <div className="design-preview-tile">
                  <span className="design-preview-label">Dark preview</span>
                  <span className="design-preview-swatch design-preview-swatch--dark" />
                </div>
              </div>

              <div className="design-ui-font-row">
                <label htmlFor="design-ui-font-size">Interface font size</label>
                <p className="settings-description">
                  Sidebar, buttons, composer, settings, and other controls — not chat message text.
                </p>
                <div className="design-message-font-control">
                  <input
                    id="design-ui-font-size"
                    type="range"
                    className="design-message-font-slider"
                    min={UI_FONT_SIZE_SLIDER_MIN_PX}
                    max={UI_FONT_SIZE_SLIDER_MAX_PX}
                    step={1}
                    value={uiFontSizePx}
                    onChange={e => setUiFontSizePx(Number(e.target.value))}
                    aria-valuemin={UI_FONT_SIZE_SLIDER_MIN_PX}
                    aria-valuemax={UI_FONT_SIZE_SLIDER_MAX_PX}
                    aria-valuenow={uiFontSizePx}
                  />
                  <span className="design-message-font-value">{uiFontSizePx}px</span>
                </div>
              </div>

              <div className="design-message-font-row">
                <label htmlFor="design-message-font-size">Chat message font size</label>
                <div className="design-message-font-control">
                  <input
                    id="design-message-font-size"
                    type="range"
                    className="design-message-font-slider"
                    min={MESSAGE_FONT_SIZE_SLIDER_MIN_PX}
                    max={MESSAGE_FONT_SIZE_SLIDER_MAX_PX}
                    step={1}
                    value={messageFontSizePx}
                    onChange={e => setMessageFontSizePx(Number(e.target.value))}
                    aria-valuemin={MESSAGE_FONT_SIZE_SLIDER_MIN_PX}
                    aria-valuemax={MESSAGE_FONT_SIZE_SLIDER_MAX_PX}
                    aria-valuenow={messageFontSizePx}
                  />
                  <span className="design-message-font-value">{messageFontSizePx}px</span>
                </div>
              </div>

              <button type="button" className="design-reset-btn" onClick={resetDesignColors}>
                Reset to defaults
              </button>
            </div>
          )}
        </div>

        {/* Footer Buttons */}
        <div className="modal-buttons">
          <button className="save-btn" onClick={save} disabled={saving || !hasUnsavedChanges}>
            {saving ? 'Saving…' : 'Save'}
          </button>
          <button type="button" onClick={handleClose}>
            {hasUnsavedChanges ? 'Cancel' : 'Close'}
          </button>
        </div>
      </div>
    </div>
  )
}
