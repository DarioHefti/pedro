import { useState, useEffect, useCallback } from 'react'
import { conversationService, type Conversation } from '../services/wailsService'

/**
 * Manages the conversation list state and CRUD operations.
 * Does NOT manage which conversation is currently selected — that lives in App
 * so it can coordinate with the messaging hook.
 */
export function useConversations() {
  const [conversations, setConversations] = useState<Conversation[]>([])

  const load = useCallback(async (): Promise<void> => {
    const convs = await conversationService.getAll()
    setConversations(convs ?? [])
  }, [])

  const remove = useCallback(
    async (id: number): Promise<void> => {
      await conversationService.delete(id)
      await load()
    },
    [load],
  )

  const removeAll = useCallback(async (): Promise<void> => {
    await conversationService.deleteAll()
    await load()
  }, [load])

  useEffect(() => {
    load()
  }, [load])

  return { conversations, load, remove, removeAll }
}
