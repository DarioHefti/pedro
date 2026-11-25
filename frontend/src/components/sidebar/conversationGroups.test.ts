import { expect, it } from 'vitest'
import type { Conversation } from '../../services/wailsService'
import { groupConversations } from './conversationGroups'

const now = new Date(2026, 10, 24, 12, 0)
const at = (daysAgo: number, hour = 9) => new Date(2026, 10, 24 - daysAgo, hour).toISOString()
const conv = (ID: number, UpdatedAt: string): Conversation => ({ ID, Title: `Chat ${ID}`, CreatedAt: '', UpdatedAt }) as Conversation

it('groups chats into recency buckets in the order given', () => {
  const groups = groupConversations(
    [conv(1, at(0)), conv(2, at(1, 23)), conv(3, at(3)), conv(4, at(0, 1)), conv(5, at(12)), conv(6, at(45)), conv(7, 'not a date')],
    now,
  )
  expect(groups.map(g => [g.label, g.conversations.map(c => c.ID)])).toEqual([
    ['Today', [1, 4]],
    ['Yesterday', [2]],
    ['Previous 7 days', [3]],
    ['Previous 30 days', [5]],
    ['Older', [6, 7]],
  ])
})

it('falls back to the creation time when a chat was never updated', () => {
  const [group] = groupConversations([{ ID: 1, Title: 'x', CreatedAt: at(1), UpdatedAt: '' } as Conversation], now)
  expect(group.label).toBe('Yesterday')
})
