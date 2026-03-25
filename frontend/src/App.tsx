import { useState, useEffect } from 'react'
import Sidebar from './Sidebar'
import Chat from './Chat'
import SettingsModal from './SettingsModal'
import Toaster from './components/Toaster'
import { useConversations } from './hooks/useConversations'
import { useMessaging } from './hooks/useMessaging'
import { useToast } from './context/ToastContext'
import {
  conversationService,
  settingsService,
  fileService,
  MOCK_EMPTY_CHAT_UI,
  MOCK_UI_CONVERSATION_ID,
} from './services/wailsService'
import { applyDesignAndTypographyFromSettings } from './designTheme'

const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'

export default function App() {
  const toast = useToast()
  const [currentConvID, setCurrentConvID] = useState<number | null>(() =>
    MOCK_EMPTY_CHAT_UI ? MOCK_UI_CONVERSATION_ID : null,
  )
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)

  const { conversations, load: loadConversations, remove: removeConversation } =
    useConversations()

  const messaging = useMessaging({
    currentConvID,
    createConversation: conversationService.create,
    onConversationCreated: setCurrentConvID,
    refreshConversations: loadConversations,
    showError: toast.error,
  })

  useEffect(() => {
    settingsService.get().then(s => {
      setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
      applyDesignAndTypographyFromSettings(s)
    })
  }, [])

  useEffect(() => {
    if (!MOCK_EMPTY_CHAT_UI) return
    if (currentConvID !== MOCK_UI_CONVERSATION_ID) return
    void messaging.load(MOCK_UI_CONVERSATION_ID)
    // Bootstrap sample thread once when opening on the virtual conversation.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional
  }, [])

  // Explicit: load messages only when the user actively selects a conversation.
  // NOT driven by a useEffect so that programmatic setCurrentConvID calls (e.g.
  // during send for a new conversation) never race with the optimistic update.
  async function selectConversation(id: number) {
    setCurrentConvID(id)
    await messaging.load(id)
  }

  function newConversation() {
    setCurrentConvID(null)
    messaging.clear()
  }

  async function deleteConversation(id: number) {
    await removeConversation(id)
    if (currentConvID === id) {
      setCurrentConvID(null)
      messaging.clear()
    }
  }

  function handleSettingsClose() {
    setSettingsOpen(false)
    // Reload welcome message in case it was changed
    settingsService.get().then(s => {
      setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
      applyDesignAndTypographyFromSettings(s)
    })
  }

  return (
    <div className="layout">
      <Sidebar
        conversations={conversations}
        currentConvID={currentConvID}
        onSelect={selectConversation}
        onNew={newConversation}
        onDelete={deleteConversation}
        onOpenSettings={() => setSettingsOpen(true)}
        onSearch={conversationService.search}
      />
      <Chat
        messages={messaging.messages}
        loading={messaging.loading}
        toolCalls={messaging.toolCalls}
        streamingContent={messaging.streamingContent}
        messageImages={messaging.messageImages}
        messageFiles={messaging.messageFiles}
        onSend={messaging.send}
        onStop={messaging.stop}
        onRegenerate={messaging.regenerate}
        onSelectFile={fileService.selectFile}
        onSelectFolder={fileService.selectFolder}
        welcomeMessage={welcomeMessage}
      />
      {settingsOpen && (
        <SettingsModal
          onClose={handleSettingsClose}
          onSaved={() => {
            settingsService.get().then(s => {
              setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
              applyDesignAndTypographyFromSettings(s)
            })
          }}
          getSettings={settingsService.get}
          saveSettings={settingsService.save}
          setSetting={settingsService.setSetting}
          testConnection={settingsService.test}
          signIn={settingsService.signIn}
          signOut={settingsService.signOut}
          isAuthenticated={settingsService.isAuthenticated}
        />
      )}
      <Toaster />
    </div>
  )
}
