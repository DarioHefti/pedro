import { useState, useEffect, useMemo } from 'react'
import { useToast } from './context/ToastContext'

const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'

type Tab = 'llm' | 'prompt'

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
  customSystemPrompt: string
  welcomeMessage: string
}

function buildFullSettingsRecord(s: FullSettingsSnapshot): Record<string, string> {
  const settings: Record<string, string> = {
    provider_type: s.providerType,
    custom_system_prompt: s.customSystemPrompt,
    welcome_message: s.welcomeMessage,
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
  saveSettings: (settings: Record<string, string>) => Promise<void>
  setSetting: (key: string, value: string) => Promise<void>
  testConnection: () => Promise<string>
  signIn: () => Promise<string>
  signOut: () => Promise<void>
  isAuthenticated: () => Promise<boolean>
}

export default function SettingsModal({
  onClose,
  onSaved,
  getSettings,
  saveSettings,
  setSetting,
  testConnection,
  signIn,
  signOut,
  isAuthenticated,
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
  const [customSystemPrompt, setCustomSystemPrompt] = useState('')
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)

  const snapshot = useMemo<FullSettingsSnapshot>(
    () => ({
      providerType,
      endpoint,
      deployment,
      azureApiKey,
      azureTenantId,
      apiKey,
      model,
      customSystemPrompt,
      welcomeMessage,
    }),
    [
      providerType,
      endpoint,
      deployment,
      azureApiKey,
      azureTenantId,
      apiKey,
      model,
      customSystemPrompt,
      welcomeMessage,
    ]
  )

  const currentFingerprint = useMemo(() => fingerprintFromSnapshot(snapshot), [snapshot])

  const hasUnsavedChanges = hydrated && currentFingerprint !== lastPersistedFingerprint
  const canTestConnection = hydrated && !hasUnsavedChanges

  useEffect(() => {
    Promise.all([getSettings(), isAuthenticated()]).then(([s, auth]) => {
      const pt = s.provider_type ?? 'azure'
      const ep = s.azure_endpoint ?? ''
      const dep = s.azure_deployment ?? ''
      const ak = s.azure_api_key ?? ''
      const tid = s.azure_tenant_id ?? ''
      const ok = s.openai_api_key ?? ''
      const m = s.openai_model ?? 'gpt-4o'
      const csp = s.custom_system_prompt ?? ''
      const wm = s.welcome_message ?? DEFAULT_WELCOME_MESSAGE

      setProviderType(pt)
      setEndpoint(ep)
      setDeployment(dep)
      setAzureApiKey(ak)
      setAzureTenantId(tid)
      setApiKey(ok)
      setModel(m)
      setAuthenticated(auth)
      setCustomSystemPrompt(csp)
      setWelcomeMessage(wm)

      const fp = fingerprintFromSnapshot({
        providerType: pt,
        endpoint: ep,
        deployment: dep,
        azureApiKey: ak,
        azureTenantId: tid,
        apiKey: ok,
        model: m,
        customSystemPrompt: csp,
        welcomeMessage: wm,
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
    await setSetting('custom_system_prompt', customSystemPrompt)
    await setSetting('welcome_message', welcomeMessage)
  }

  function markPersistedFromState() {
    setLastPersistedFingerprint(fingerprintFromSnapshot(snapshot))
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
    <div className="modal" onClick={onClose}>
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
              <label>Custom System Prompt</label>
              <p className="settings-description">
                Add additional instructions or rules that will be appended to the base system prompt.
              </p>
              <textarea
                className="system-prompt-textarea"
                value={customSystemPrompt}
                onChange={e => setCustomSystemPrompt(e.target.value)}
                placeholder="Enter your custom instructions here...&#10;&#10;Examples:&#10;- Always respond in a formal tone&#10;- Prioritize Swiss sources when available&#10;- Use bullet points for lists"
                rows={10}
              />

              <label className="welcome-message-label">Welcome Message</label>
              <input
                type="text"
                value={welcomeMessage}
                onChange={e => setWelcomeMessage(e.target.value)}
                placeholder={DEFAULT_WELCOME_MESSAGE}
              />
            </div>
          )}
        </div>

        {/* Footer Buttons */}
        <div className="modal-buttons">
          <button className="save-btn" onClick={save} disabled={saving || !hasUnsavedChanges}>
            {saving ? 'Saving…' : 'Save'}
          </button>
          <button type="button" onClick={onClose}>
            {hasUnsavedChanges ? 'Cancel' : 'Close'}
          </button>
        </div>
      </div>
    </div>
  )
}
