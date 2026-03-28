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
  personaService,
  uiConversationService,
} from './services/wailsService'
import type { Persona } from './services/wailsService'
import { applyDesignAndTypographyFromSettings } from './designTheme'

const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'

export default function App() {
  const toast = useToast()
  const [currentConvID, setCurrentConvID] = useState<number | null>(() =>
    uiConversationService.initialConversationID(),
  )
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)
  const [personas, setPersonas] = useState<Persona[]>([])
  const [activePersonaId, setActivePersonaId] = useState<string>('')
  const [focusTrigger, setFocusTrigger] = useState(0)

  const {
    conversations,
    load: loadConversations,
    remove: removeConversation,
    removeAll: removeAllConversations,
  } = useConversations()

  async function reloadSettings() {
    const s = await settingsService.get()
    setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
    applyDesignAndTypographyFromSettings(s)
    try {
      const [list, active] = await Promise.all([
        personaService.getAll(),
        personaService.getActiveId(),
      ])
      setPersonas(list ?? [])
      setActivePersonaId(active ?? '')
    } catch {
      setPersonas([])
      setActivePersonaId('')
    }
  }

  const messaging = useMessaging({
    currentConvID,
    createConversation: conversationService.create,
    onConversationCreated: setCurrentConvID,
    refreshConversations: loadConversations,
    showError: toast.error,
  })

  useEffect(() => {
    void reloadSettings()
  }, [])

  useEffect(() => {
    if (!uiConversationService.isVirtualConversation(currentConvID)) return
    if (currentConvID === null) return
    void messaging.load(currentConvID)
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
    setFocusTrigger(n => n + 1)
  }

  async function deleteConversation(id: number) {
    await removeConversation(id)
    if (currentConvID === id) {
      setCurrentConvID(null)
      messaging.clear()
    }
  }

  async function deleteAllChats() {
    try {
      await removeAllConversations()
      setCurrentConvID(null)
      messaging.clear()
    } catch (err) {
      toast.error('Failed to delete all chats: ' + String(err))
    }
  }

  function handleSettingsClose() {
    setSettingsOpen(false)
    // Reload app-level values that can be changed in settings.
    void reloadSettings()
  }

  return (
    <div className="layout">
      <Sidebar
        conversations={conversations}
        currentConvID={currentConvID}
        onSelect={selectConversation}
        onNew={newConversation}
        onDelete={deleteConversation}
        onDeleteAllChats={deleteAllChats}
        streamingBusy={messaging.streamingBusy}
        onOpenSettings={() => setSettingsOpen(true)}
        onSearch={conversationService.search}
      />
      <Chat
        messages={messaging.messages}
        loading={messaging.loading}
        streamingBusy={messaging.streamingBusy}
        toolCalls={messaging.toolCalls}
        messageToolCalls={messaging.messageToolCalls}
        streamingContent={messaging.streamingContent}
        messageImages={messaging.messageImages}
        messageFiles={messaging.messageFiles}
        onSend={(content, attachments, selectedPersonaId) =>
          void messaging.send(content, attachments, selectedPersonaId)
        }
        onStop={messaging.stop}
        onRegenerate={(index, selectedPersonaId) =>
          void messaging.regenerate(index, selectedPersonaId)
        }
        onSelectFile={fileService.selectFile}
        onSelectFolder={fileService.selectFolder}
        welcomeMessage={welcomeMessage}
        personas={personas}
        activePersonaId={activePersonaId}
        onPersonaChange={async (id: string) => {
          try {
            await personaService.setActive(id)
            setActivePersonaId(id)
          } catch (err) {
            toast.error('Failed to set persona: ' + String(err))
          }
        }}
        focusTrigger={focusTrigger}
      />
      {settingsOpen && (
        <SettingsModal
          onClose={handleSettingsClose}
          onSaved={() => {
            void reloadSettings()
          }}
          getSettings={settingsService.get}
          getDefaultSystemPrompt={settingsService.getDefaultSystemPrompt}
          saveSettings={settingsService.save}
          setSetting={settingsService.setSetting}
          testConnection={settingsService.test}
          signIn={settingsService.signIn}
          signOut={settingsService.signOut}
          isAuthenticated={settingsService.isAuthenticated}
          getPersonas={personaService.getAll}
          createPersona={personaService.create}
          updatePersona={personaService.update}
          deletePersona={personaService.delete}
        />
      )}
      <Toaster />
    </div>
  )
}
