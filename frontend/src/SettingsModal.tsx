import { useState, useEffect } from 'react'
import { useToast } from './context/ToastContext'

const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'

type Tab = 'llm' | 'prompt'

interface SettingsModalProps {
  onClose: () => void
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
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState('gpt-4o')
  const [authenticated, setAuthenticated] = useState(false)
  const [signingIn, setSigningIn] = useState(false)
  const [signingOut, setSigningOut] = useState(false)
  const [testing, setTesting] = useState(false)

  // Prompt Settings
  const [customSystemPrompt, setCustomSystemPrompt] = useState('')
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)

  useEffect(() => {
    Promise.all([getSettings(), isAuthenticated()]).then(([s, auth]) => {
      setProviderType(s.provider_type ?? 'azure')
      setEndpoint(s.azure_endpoint ?? s.openai_endpoint ?? '')
      setDeployment(s.azure_deployment ?? '')
      setApiKey(s.openai_api_key ?? '')
      setModel(s.openai_model ?? 'gpt-4o')
      setAuthenticated(auth)
      setCustomSystemPrompt(s.custom_system_prompt ?? '')
      setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  async function save() {
    const settings: Record<string, string> = {
      provider_type: providerType,
    }

    if (providerType === 'azure') {
      settings.azure_endpoint = endpoint
      settings.azure_deployment = deployment
    } else if (providerType === 'openai') {
      settings.openai_api_key = apiKey
      settings.openai_model = model
    }

    await saveSettings(settings)
    await setSetting('custom_system_prompt', customSystemPrompt)
    await setSetting('welcome_message', welcomeMessage)
    toast.success('Settings saved!')
    onClose()
  }

  async function handleSignIn() {
    setSigningIn(true)
    try {
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
      await signOut()
      setAuthenticated(false)
      setEndpoint('')
      setDeployment('')
      toast.success('Signed out')
      onClose()
    } catch (err) {
      toast.error('Sign out error: ' + String(err))
    } finally {
      setSigningOut(false)
    }
  }

  async function test() {
    setTesting(true)
    try {
      const result = await testConnection()
      if (result.startsWith('Error:')) {
        toast.error(result)
      } else {
        // testConnection may have triggered sign-in
        const auth = await isAuthenticated()
        setAuthenticated(auth)
        toast.success(result)
      }
    } catch (err) {
      toast.error('Error: ' + String(err))
    } finally {
      setTesting(false)
    }
  }

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
              <label>Provider</label>
              <select
                value={providerType}
                onChange={e => setProviderType(e.target.value)}
                className="provider-select"
              >
                <option value="azure">Azure OpenAI</option>
                <option value="openai">OpenAI</option>
              </select>

              {providerType === 'azure' && (
                <>
                  <label>Endpoint URL</label>
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
                {authenticated ? (
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
                    {signingIn ? 'Opening browser...' : providerType === 'azure' ? 'Sign In with Azure' : 'Sign In'}
                  </button>
                )}
                <button className="test-btn" onClick={test} disabled={testing}>
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
          <button className="save-btn" onClick={save}>
            Save
          </button>
          <button onClick={onClose}>Cancel</button>
        </div>
      </div>
    </div>
  )
}
