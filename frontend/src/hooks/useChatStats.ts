import { useCallback, useEffect, useRef, useState } from 'react'
import { eventService, statsService } from '../services/wailsService'

export interface ChatStats {
  requests: number
  tokens: number
}

const NO_STATS: ChatStats = { requests: 0, tokens: 0 }

/**
 * LLM request and token totals for the open chat. Live updates arrive as the backend finishes
 * each request; `load` fetches the stored totals when a chat is opened.
 */
export function useChatStats(conversationID: number | null) {
  const [stats, setStats] = useState<ChatStats>(NO_STATS)
  /** The chat whose totals were last requested; an older answer arriving late is dropped. */
  const requestedRef = useRef<number | null>(null)

  useEffect(
    () =>
      eventService.on('request_count_updated', (convID: number, perChat: number, _global: number, chatTokens: number) => {
        if (convID === conversationID) setStats({ requests: perChat, tokens: chatTokens })
      }),
    [conversationID],
  )

  const load = useCallback(async (id: number) => {
    requestedRef.current = id
    let next = NO_STATS
    try {
      const counts = await statsService.getRequestCounts(id)
      next = { requests: counts.perChat, tokens: counts.perChatTokens }
    } catch {
      /* totals are informational; show zero */
    }
    if (requestedRef.current === id) setStats(next)
  }, [])

  const reset = useCallback(() => {
    requestedRef.current = null
    setStats(NO_STATS)
  }, [])

  return { stats, load, reset }
}
