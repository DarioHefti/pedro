import { useState, useEffect } from 'react'

interface SettingsModalProps {
  onClose: () => void
  GetSettings: () => Promise<Record<string, string>>
  SaveSettings: (endpoint: string, apiKey: string, deployment: string) => Promise<void>
  TestConnection: (endpoint: string, apiKey: string, deployment: string) => Promise<string>
}

export default function SettingsModal({
  onClose,
  GetSettings,
  SaveSettings,
  TestConnection,
}: SettingsModalProps) {
  const [endpoint, setEndpoint] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [deployment, setDeployment] = useState('')
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    GetSettings().then(s => {
      setEndpoint(s.azure_endpoint ?? '')
      setApiKey(s.azure_api_key ?? '')
      setDeployment(s.azure_deployment ?? '')
    })
  }, [])

  async function save() {
    await SaveSettings(endpoint, apiKey, deployment)
    onClose()
    alert('Settings saved!')
  }

  async function test() {
    setTesting(true)
    try {
      const result = await TestConnection(endpoint, apiKey, deployment)
      alert(result)
    } catch (err) {
      alert('Error: ' + String(err))
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
        <div className="modal-buttons">
          <button className="test-btn" onClick={test} disabled={testing}>
            {testing ? 'Testing...' : 'Test Connection'}
          </button>
          <button className="save-btn" onClick={save}>Save</button>
          <button onClick={onClose}>Cancel</button>
        </div>
      </div>
    </div>
  )
}