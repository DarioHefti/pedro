import type { Conversation } from '../../services/wailsService'

export interface ConversationGroup {
  key: string
  label: string
  conversations: Conversation[]
}

const DAY_MS = 86_400_000

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

function bucketOf(conv: Conversation, today: number): { key: string; label: string } {
  const raw = conv.UpdatedAt || conv.CreatedAt
  const date = raw ? new Date(raw) : null
  if (!date || Number.isNaN(date.getTime())) return { key: 'older', label: 'Older' }
  const days = Math.round((today - startOfDay(date)) / DAY_MS)
  if (days <= 0) return { key: 'today', label: 'Today' }
  if (days === 1) return { key: 'yesterday', label: 'Yesterday' }
  if (days < 7) return { key: 'week', label: 'Previous 7 days' }
  if (days < 30) return { key: 'month', label: 'Previous 30 days' }
  return { key: 'older', label: 'Older' }
}

/** Groups conversations (kept in the order given) into recency buckets. */
export function groupConversations(conversations: Conversation[], now = new Date()): ConversationGroup[] {
  const today = startOfDay(now)
  const groups: ConversationGroup[] = []
  const byKey = new Map<string, ConversationGroup>()
  for (const conv of conversations) {
    const { key, label } = bucketOf(conv, today)
    let group = byKey.get(key)
    if (!group) {
      group = { key, label, conversations: [] }
      byKey.set(key, group)
      groups.push(group)
    }
    group.conversations.push(conv)
  }
  return groups
}
