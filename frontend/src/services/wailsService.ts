/**
 * Central service layer wrapping all Wails bridge calls.
 *
 * This is the ONLY place in the frontend that imports from the auto-generated
 * wailsjs directory. All components and hooks must go through these services
 * so that the Wails coupling is isolated to a single, swappable boundary.
 */
import {
  GetConversations,
  GetMessages,
  CreateConversation,
  DeleteConversation,
  SearchMessages,
  SendMessage,
  SendMessageWithImages,
  RegenerateMessage,
  AbortMessage,
  GetSettings,
  SaveSettings,
  SetSetting,
  TestConnection,
  SelectFile,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { main } from '../../wailsjs/go/models'

// Re-export generated types so the rest of the app never imports from wailsjs.
export type Conversation = main.Conversation
export type Message = main.Message

// ---------------------------------------------------------------------------
// Conversation service
// ---------------------------------------------------------------------------
export const conversationService = {
  getAll: (): Promise<Conversation[]> => GetConversations(),
  getMessages: (id: number): Promise<Message[]> => GetMessages(id),
  create: (): Promise<Conversation> => CreateConversation(),
  delete: (id: number): Promise<void> => DeleteConversation(id),
  search: (query: string): Promise<Record<number, Message[]>> => SearchMessages(query),
}

// ---------------------------------------------------------------------------
// Message service
// ---------------------------------------------------------------------------
export const messageService = {
  send: (convID: number, content: string): Promise<string> =>
    SendMessage(convID, content),
  sendWithImages: (
    convID: number,
    content: string,
    images: string[],
  ): Promise<string> => SendMessageWithImages(convID, content, images),
  regenerate: (convID: number): Promise<string> => RegenerateMessage(convID),
  abort: (): Promise<void> => AbortMessage(),
}

// ---------------------------------------------------------------------------
// Settings service
// ---------------------------------------------------------------------------
export const settingsService = {
  get: (): Promise<Record<string, string>> => GetSettings(),
  save: (
    endpoint: string,
    apiKey: string,
    deployment: string,
  ): Promise<void> => SaveSettings(endpoint, apiKey, deployment),
  setSetting: (key: string, value: string): Promise<void> => SetSetting(key, value),
  test: (
    endpoint: string,
    apiKey: string,
    deployment: string,
  ): Promise<string> => TestConnection(endpoint, apiKey, deployment),
}

// ---------------------------------------------------------------------------
// File service
// ---------------------------------------------------------------------------
export const fileService = {
  select: (): Promise<string> => SelectFile(),
}

// ---------------------------------------------------------------------------
// Event service (streaming)
// ---------------------------------------------------------------------------
export const eventService = {
  on: EventsOn,
  off: EventsOff,
}
