import { useState, useEffect } from 'react'
import { useToast } from './context/ToastContext'

const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'

interface SettingsModalProps {
  onClose: () => void
  getSettings: () => Promise<Record<string, string>>
  saveSettings: (endpoint: string, apiKey: string, deployment: string) => Promise<void>
  setSetting: (key: string, value: string) => Promise<void>
  testConnection: (
    endpoint: string,
    apiKey: string,
    deployment: string,
  ) => Promise<string>
}

export default function SettingsModal({
  onClose,
  getSettings,
  saveSettings,
  setSetting,
  testConnection,
}: SettingsModalProps) {
  const toast = useToast()
  const [endpoint, setEndpoint] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [deployment, setDeployment] = useState('')
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    getSettings().then(s => {
      setEndpoint(s.azure_endpoint ?? '')
      setApiKey(s.azure_api_key ?? '')
      setDeployment(s.azure_deployment ?? '')
      setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  async function save() {
    await saveSettings(endpoint, apiKey, deployment)
    await setSetting('welcome_message', welcomeMessage)
    toast.success('Settings saved!')
    onClose()
  }

  async function test() {
    setTesting(true)
    try {
      const result = await testConnection(endpoint, apiKey, deployment)
      if (result.startsWith('Error:')) {
        toast.error(result)
      } else {
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
      <div className="modal-content" onClick={e => e.stopPropagation()}>
        <h2>Azure AI Settings</h2>
        <label>Endpoint URL</label>
        <input
          type="text"
          value={endpoint}
          onChange={e => setEndpoint(e.target.value)}
          placeholder="https://your-resource.openai.azure.com"
        />
        <label>API Key</label>
        <input
          type="password"
          value={apiKey}
          onChange={e => setApiKey(e.target.value)}
          placeholder="Your Azure API key"
        />
        <label>Deployment Name</label>
        <input
          type="text"
          value={deployment}
          onChange={e => setDeployment(e.target.value)}
          placeholder="gpt-4"
        />
        <label>Welcome Message</label>
        <input
          type="text"
          value={welcomeMessage}
          onChange={e => setWelcomeMessage(e.target.value)}
          placeholder={DEFAULT_WELCOME_MESSAGE}
        />
        <div className="modal-buttons">
          <button className="test-btn" onClick={test} disabled={testing}>
            {testing ? 'Testing...' : 'Test Connection'}
          </button>
          <button className="save-btn" onClick={save}>
            Save
          </button>
          <button onClick={onClose}>Cancel</button>
        </div>
      </div>
    </div>
  )
}
