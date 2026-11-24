import { useCallback, useEffect, useRef, useState } from 'react'
import { Resizable, ResizableHandle, ResizablePanel, createResizableController } from '@ui-sdk/react/resizable'
import { useShortcut } from '@ui-sdk/react/shortcuts'
import Chat from './components/chat/Chat'
import { CommandPalette, type PaletteAction } from './components/CommandPalette'
import { DiagramViewer } from './components/diagram/DiagramViewer'
import { ImageLightbox } from './components/ImageLightbox'
import { useCodeBlockDownloads } from './components/media/useCodeBlockDownloads'
import Sidebar from './components/sidebar/Sidebar'
import UpdateNotification from './components/UpdateNotification'
import Details from './Details'
import { applyDesignAndTypographyFromSettings } from './designTheme'
import { useChatStats } from './hooks/useChatStats'
import { useConversations } from './hooks/useConversations'
import { useMessaging } from './hooks/useMessaging'
import { usePinnedSidebar } from './hooks/usePinnedSidebar'
import {
  conversationService,
  fileService,
  memoryService,
  settingsService,
  statsService,
  updaterService,
} from './services/wailsService'
import { SettingsDialog } from './settings/SettingsDialog'
import { DEFAULT_WELCOME_MESSAGE, type SettingsServices } from './settings/settingsModel'
import { SHORTCUTS } from './shortcuts'
import { useTheme } from './ThemeContext'
import { ConfirmHost } from './ui/confirm'
import { Toaster, toast } from './ui/toaster'

const SIDEBAR_ID = 'sidebar'
/** Sidebar widths in pixels: the sidebar keeps its width as the window resizes. */
const SIDEBAR_WIDTH = { default: 272, min: 220, max: 420 }

const settingsServices: SettingsServices = {
  getSettings: settingsService.get,
  getDefaultSystemPrompt: settingsService.getDefaultSystemPrompt,
  saveSettings: settingsService.save,
  setSetting: settingsService.setSetting,
  testConnection: settingsService.test,
  signIn: settingsService.signIn,
  signOut: settingsService.signOut,
  isAuthenticated: settingsService.isAuthenticated,
  getMemories: memoryService.getAll,
  saveMemory: memoryService.save,
  deleteMemory: memoryService.delete,
  getLifetimeStats: statsService.getLifetimeStats,
  getCurrentVersion: updaterService.getCurrentVersion,
}

const showError = (message: string) => toast.error(message)

export default function App() {
  const { theme, toggleTheme } = useTheme()
  const [currentConvID, setCurrentConvID] = useState<number | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [welcomeMessage, setWelcomeMessage] = useState(DEFAULT_WELCOME_MESSAGE)
  const [focusTrigger, setFocusTrigger] = useState(0)
  const [layout] = useState(() => createResizableController())
  const shellRef = useRef<HTMLDivElement>(null)
  const sidebar = usePinnedSidebar({
    controller: layout,
    panelId: SIDEBAR_ID,
    groupRef: shellRef,
    defaultWidth: SIDEBAR_WIDTH.default,
    minWidth: SIDEBAR_WIDTH.min,
    maxWidth: SIDEBAR_WIDTH.max,
    storageKey: 'pedro-sidebar',
  })

  const { conversations, load: loadConversations, remove: removeConversation, removeAll: removeAllConversations } =
    useConversations()
  const chatStats = useChatStats(currentConvID)
  useCodeBlockDownloads()

  const reloadSettings = useCallback(async () => {
    const s = await settingsService.get()
    setWelcomeMessage(s.welcome_message ?? DEFAULT_WELCOME_MESSAGE)
    applyDesignAndTypographyFromSettings(s)
  }, [])

  /** Leaves a conversation that no longer exists. */
  const forgetConversation = useCallback((id: number) => {
    setCurrentConvID(current => (current === id ? null : current))
  }, [])

  const messaging = useMessaging({
    currentConvID,
    createConversation: conversationService.create,
    onConversationCreated: setCurrentConvID,
    refreshConversations: loadConversations,
    onConversationDeleted: forgetConversation,
    showError,
  })

  useEffect(() => {
    void reloadSettings()
  }, [reloadSettings])

  // Loads messages only when the person picks a conversation, so programmatic setCurrentConvID
  // during a send never races the optimistic update.
  async function selectConversation(id: number) {
    setCurrentConvID(id)
    setFocusTrigger(n => n + 1)
    void chatStats.load(id)
    await messaging.load(id)
  }

  function newConversation() {
    setCurrentConvID(null)
    messaging.clear()
    chatStats.reset()
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
    await removeAllConversations()
    setCurrentConvID(null)
    messaging.clear()
  }

  function runPaletteAction(action: PaletteAction) {
    if (action === 'new-chat') newConversation()
    else if (action === 'settings') setSettingsOpen(true)
    else if (action === 'details') setDetailsOpen(true)
    else if (action === 'toggle-theme') toggleTheme()
    else if (action === 'toggle-sidebar') sidebar.toggle()
  }

  useShortcut(SHORTCUTS.commandPalette, () => setPaletteOpen(true), { description: 'Command palette' })
  useShortcut(SHORTCUTS.newChat, newConversation, { description: 'New chat' })
  useShortcut(SHORTCUTS.newChatAlt, newConversation, { description: 'New chat' })
  useShortcut(SHORTCUTS.toggleSidebar, sidebar.toggle, { description: 'Toggle sidebar' })
  useShortcut(SHORTCUTS.settings, () => setSettingsOpen(true), { description: 'Settings' })
  useShortcut(SHORTCUTS.requestDetails, () => setDetailsOpen(true), { description: 'Request details' })

  const currentTitle = conversations.find(c => c.ID === currentConvID)?.Title || 'New chat'

  return (
    <div className="app">
      <Resizable
        ref={shellRef}
        className="app-shell"
        controller={layout}
        onLayoutChange={sidebar.onLayoutChange}
        onLayoutCommit={sidebar.onLayoutCommit}
      >
        <ResizablePanel
          id={SIDEBAR_ID}
          className="app-sidebar"
          defaultSize={sidebar.defaultSize}
          minSize={`${SIDEBAR_WIDTH.min}px`}
          maxSize={`${SIDEBAR_WIDTH.max}px`}
          collapsible
          collapsedSize="0px"
        >
          <Sidebar
            conversations={conversations}
            currentConvID={currentConvID}
            onSelect={selectConversation}
            onNew={newConversation}
            onDelete={deleteConversation}
            onDeleteAllChats={deleteAllChats}
            streamingBusy={messaging.streamingBusy}
            onOpenSettings={() => setSettingsOpen(true)}
            onCollapse={sidebar.collapse}
            onSearch={conversationService.search}
          />
        </ResizablePanel>
        <ResizableHandle aria-label="Resize sidebar" />
        <ResizablePanel id="main" className="app-main" minSize="420px">
          <Chat
            title={currentTitle}
            thread={messaging.thread}
            stream={messaging.stream}
            streamingBusy={messaging.streamingBusy}
            onSend={(content, attachments) => void messaging.send(content, attachments)}
            onStop={messaging.stop}
            onRegenerate={index => void messaging.regenerate(index)}
            onDeleteMessage={index => void messaging.deleteMessage(index)}
            onResendMessage={index => void messaging.resendMessage(index)}
            onSelectFile={fileService.selectFile}
            onSelectFolder={fileService.selectFolder}
            welcomeMessage={welcomeMessage}
            focusTrigger={focusTrigger}
            currentConvID={currentConvID}
            composerFocusPaused={settingsOpen || detailsOpen || paletteOpen}
            stats={chatStats.stats}
            onViewDetails={() => setDetailsOpen(true)}
            sidebarCollapsed={sidebar.collapsed}
            onShowSidebar={sidebar.expand}
            onNewChat={newConversation}
            onOpenPalette={() => setPaletteOpen(true)}
          />
        </ResizablePanel>
      </Resizable>

      <SettingsDialog
        open={settingsOpen}
        onClose={() => {
          setSettingsOpen(false)
          void reloadSettings()
        }}
        onSaved={() => void reloadSettings()}
        services={settingsServices}
        onViewDetails={() => setDetailsOpen(true)}
      />
      <Details open={detailsOpen} onClose={() => setDetailsOpen(false)} conversationID={currentConvID} />
      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        conversations={conversations}
        theme={theme}
        onAction={runPaletteAction}
        onSelectConversation={id => void selectConversation(id)}
        onSearchMessages={conversationService.search}
      />
      <ImageLightbox />
      <DiagramViewer />
      <ConfirmHost />
      <Toaster />
      <UpdateNotification />
    </div>
  )
}
