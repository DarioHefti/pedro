import { useState, useEffect, useRef } from 'react'
import Sidebar from './Sidebar'
import Chat from './Chat'
import SettingsModal from './SettingsModal'
import {
  GetConversations,
  GetMessages,
  CreateConversation,
  DeleteConversation,
  SendMessage,
  SendMessageWithImages,
  RegenerateMessage,
  GetSettings,
  SaveSettings,
  TestConnection,
} from '../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import { main } from '../wailsjs/go/models'

export interface ToolCall {
  name: string
  argsJSON: string
}

export default function App() {
  const [conversations, setConversations] = useState<main.Conversation[]>([])
  const [currentConvID, setCurrentConvID] = useState<number | null>(null)
  const [messages, setMessages] = useState<main.Message[]>([])
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([])
  const [streamingContent, setStreamingContent] = useState<string>('')
  // Maps message array index -> image data URLs for that user message (session-only, not persisted)
  const [messageImages, setMessageImages] = useState<Map<number, string[]>>(new Map())
  const toolCallsRef = useRef<ToolCall[]>([])
  const streamingContentRef = useRef<string>('')

  useEffect(() => {
    loadConversations()
  }, [])

  async function loadConversations(): Promise<main.Conversation[]> {
    const convs = await GetConversations()
    setConversations(convs ?? [])
    return convs ?? []
  }

  async function selectConversation(id: number) {
    setCurrentConvID(id)
    setMessageImages(new Map()) // clear session images when switching conv
    const msgs = await GetMessages(id)
    setMessages(msgs ?? [])
  }

  function newConversation() {
    setCurrentConvID(null)
    setMessages([])
    setMessageImages(new Map())
  }

  async function deleteConversation(id: number) {
    await DeleteConversation(id)
    await loadConversations()
    if (currentConvID === id) {
      setCurrentConvID(null)
      setMessages([])
      setMessageImages(new Map())
    }
  }

  async function sendMessage(content: string, attachments?: { type: string; content: string; name: string }[]) {
    let convID = currentConvID

    if (!convID) {
      const conv = await CreateConversation()
      convID = conv.ID
      setCurrentConvID(convID)
      await loadConversations()
    }

    // Separate images from other attachments
    const imageDataURLs = (attachments ?? [])
      .filter(a => a.type === 'image')
      .map(a => a.content)

    const nonImageAttachContent = (attachments ?? [])
      .filter(a => a.type !== 'image')
      .map(a => {
        if (a.type === 'file-ref') {
          return `[File: ${a.name}]\n[Path: ${a.content}]\n[Large file attached by path. Use the read_file tool with this path to read it in chunks.]`
        }
        return `[File: ${a.name}]\n${a.content}`
      }).join('\n\n')

    const fullContent = nonImageAttachContent
      ? `${content}\n\n${nonImageAttachContent}`
      : content

    // Optimistically add the user message and record image index
    setMessages(prev => {
      const newMsgs = [...prev, { Role: 'user', Content: fullContent } as main.Message]
      if (imageDataURLs.length > 0) {
        setMessageImages(imgMap => {
          const updated = new Map(imgMap)
          updated.set(newMsgs.length - 1, imageDataURLs)
          return updated
        })
      }
      return newMsgs
    })

    toolCallsRef.current = []
    streamingContentRef.current = ''
    setToolCalls([])
    setStreamingContent('')
    setLoading(true)

    EventsOn('tool_call', (name: string, argsJSON: string) => {
      const tc: ToolCall = { name, argsJSON }
      toolCallsRef.current = [...toolCallsRef.current, tc]
      setToolCalls([...toolCallsRef.current])
    })

    EventsOn('stream_chunk', (chunk: string) => {
      streamingContentRef.current += chunk
      setStreamingContent(streamingContentRef.current)
    })

    try {
      let response: string
      if (imageDataURLs.length > 0) {
        response = await SendMessageWithImages(convID, fullContent, imageDataURLs)
      } else {
        response = await SendMessage(convID, fullContent)
      }

      const msgs = await GetMessages(convID)
      // Re-map images to new indices after DB reload (user messages keep same relative order)
      setMessages(prev => {
        const dbMsgs = msgs ?? []
        // Rebuild image map: find user messages that match our tracked ones
        setMessageImages(imgMap => {
          const updated = new Map<number, string[]>()
          imgMap.forEach((imgs, oldIdx) => {
            // Find the corresponding message in the new DB list by matching content
            const oldMsg = prev[oldIdx]
            if (oldMsg) {
              const newIdx = dbMsgs.findIndex(
                (m, i) => m.Role === 'user' && m.Content === oldMsg.Content &&
                  !Array.from(updated.keys()).includes(i)
              )
              if (newIdx >= 0) updated.set(newIdx, imgs)
            }
          })
          return updated
        })
        return dbMsgs
      })

      await loadConversations()

      if (response?.startsWith('Error:')) {
        alert(response)
      }
    } finally {
      EventsOff('tool_call')
      EventsOff('stream_chunk')
      streamingContentRef.current = ''
      setStreamingContent('')
      setLoading(false)
      setToolCalls([])
    }
  }

  async function handleRegenerate(index: number) {
    const msg = messages[index]
    if (msg.Role !== 'assistant' || !currentConvID) return

    setMessages(prev => prev.slice(0, index))

    toolCallsRef.current = []
    streamingContentRef.current = ''
    setToolCalls([])
    setStreamingContent('')
    setLoading(true)

    EventsOn('tool_call', (name: string, argsJSON: string) => {
      const tc: ToolCall = { name, argsJSON }
      toolCallsRef.current = [...toolCallsRef.current, tc]
      setToolCalls([...toolCallsRef.current])
    })

    EventsOn('stream_chunk', (chunk: string) => {
      streamingContentRef.current += chunk
      setStreamingContent(streamingContentRef.current)
    })

    try {
      const response = await RegenerateMessage(currentConvID)

      if (response?.startsWith('Error:')) {
        alert(response)
      } else {
        setMessages(prev => [...prev, { Role: 'assistant', Content: response } as main.Message])
      }
    } finally {
      EventsOff('tool_call')
      EventsOff('stream_chunk')
      streamingContentRef.current = ''
      setStreamingContent('')
      setLoading(false)
      setToolCalls([])
    }
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
      />
      <Chat
        messages={messages}
        loading={loading}
        toolCalls={toolCalls}
        streamingContent={streamingContent}
        messageImages={messageImages}
        onSend={sendMessage}
        onRegenerate={handleRegenerate}
      />
      {settingsOpen && (
        <SettingsModal
          onClose={() => setSettingsOpen(false)}
          GetSettings={GetSettings}
          SaveSettings={SaveSettings}
          TestConnection={TestConnection}
        />
      )}
    </div>
  )
}
