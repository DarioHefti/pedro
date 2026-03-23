import { useState, useEffect } from 'react'
import { useToast } from './context/ToastContext'

const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'

type Tab = 'llm' | 'prompt'

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
  const [activeTab, setActiveTab] = useState<Tab>('llm')
  
  // LLM Settings
  const [endpoint, setEndpoint] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [deployment, setDeployment] = useState('')
  const [testing, setTesting] = useState(false)
  
  // Prompt Settings
  const [customSystemPrompt, setCustomSystemPrompt] = useState('')
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)

  useEffect(() => {
    getSettings().then(s => {
      // LLM settings
      setEndpoint(s.azure_endpoint ?? '')
      setApiKey(s.azure_api_key ?? '')
      setDeployment(s.azure_deployment ?? '')
      
      // Prompt settings
      setCustomSystemPrompt(s.custom_system_prompt ?? '')
      setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  async function save() {
    // Save LLM settings
    await saveSettings(endpoint, apiKey, deployment)
    
    // Save prompt settings
    await setSetting('custom_system_prompt', customSystemPrompt)
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
              <div className="settings-test-row">
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
