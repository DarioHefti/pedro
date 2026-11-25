import { CODE_REPLY, DIAGRAM_REPLY, RESEARCH_REPLY } from './replies'
import type { FakeSeed, ReplyScript, SeedConversation, SeedMessage } from './types'

/** The user turn plus the backend's persisted form of a scripted reply (tool calls, results, answer). */
export function exchange(question: string, reply: ReplyScript): SeedMessage[] {
  const out: SeedMessage[] = [{ role: 'user', content: question }]
  const calls = reply.toolCalls ?? []
  if (calls.length > 0) {
    out.push({ role: 'assistant', content: '', toolCalls: calls.map((tc, i) => ({ name: tc.name, args: tc.args, id: `call_seed_${i}` })) })
    calls.forEach((tc, i) => out.push({ role: 'tool', content: tc.result ?? 'ok', toolCallId: `call_seed_${i}` }))
  }
  out.push({ role: 'assistant', content: reply.text ?? '' })
  return out
}

export function conversation(title: string, daysAgo: number, messages: SeedMessage[]): SeedConversation {
  return { title, daysAgo, messages, requestCount: messages.filter(m => m.role === 'assistant').length, requestTokens: 1200 }
}

/** Sample data for `npm run dev:mock`: a few realistic chats across the recency groups. */
export const demoSeed: FakeSeed = {
  conversations: [
    conversation('What changed in the latest Wails release?', 0, exchange('What changed in the latest Wails release? Please research it.', RESEARCH_REPLY('What changed in the latest Wails release?'))),
    conversation('Draw the sign-in flow', 1, exchange('Draw the sign-in flow as a mermaid diagram', DIAGRAM_REPLY)),
    conversation('Tiny Go HTTP server', 4, exchange('Show me code for a tiny Go HTTP server', CODE_REPLY)),
    conversation('Trip ideas for the weekend', 21, [
      { role: 'user', content: 'Trip ideas for the weekend near Zurich?' },
      { role: 'assistant', content: 'A few ideas:\n\n- **Rigi** for the view\n- **Lucerne** old town\n- **Rhine Falls** in Schaffhausen' },
    ]),
  ],
  memories: [
    { key: 'preferred_language', value: 'English', category: 'personal', importance: 4 },
    { key: 'editor', value: 'VS Code', category: 'work' },
  ],
  stats: { globalRequests: 42, lifetimeTokens: 51_200 },
}
