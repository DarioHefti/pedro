import { describe, expect, it } from 'vitest'
import fingerprintCasesJSON from '../../../testdata/connection_fingerprints.json'
import { connectionSettingsFingerprint, createFakeBackend } from './backend'
import { FakeStore, titleFromFirstMessage } from './store'

interface FingerprintCase {
  name: string
  settings: Record<string, string>
  fingerprint: string
}
// JSON imports type each case separately; they all share this shape.
const fingerprintCases = fingerprintCasesJSON as unknown as FingerprintCase[]

/**
 * The fake stands in for the Go backend in every E2E test, so it must keep behaving like it.
 * These checks pin the rules the E2E specs rely on.
 */

describe('FakeStore', () => {
  it('titles a chat from its first user message, without file notes, capped at 50 chars', () => {
    expect(titleFromFirstMessage('Summarize\n\n[File: a.pdf]\n[Path: /a.pdf]')).toBe('Summarize')
    expect(titleFromFirstMessage('x'.repeat(60))).toBe(`${'x'.repeat(50)}...`)

    const store = new FakeStore()
    const conv = store.createConversation()
    store.addMessage(conv.ID, 'assistant', 'Hi!')
    store.addMessage(conv.ID, 'user', 'First question')
    store.addMessage(conv.ID, 'user', 'Second question')
    expect(store.getConversation(conv.ID)?.Title).toBe('First question')
  })

  it('enforces the conversation foreign key like SQLite', () => {
    const store = new FakeStore()
    expect(() => store.addMessage(99, 'user', 'orphan')).toThrow('FOREIGN KEY constraint failed')
  })

  it('deletes messages from an index onwards, and cascades conversation deletes', () => {
    const store = new FakeStore()
    const a = store.createConversation()
    const b = store.createConversation()
    for (const c of ['q1', 'a1', 'q2', 'a2']) store.addMessage(a.ID, 'user', c)
    store.addMessage(b.ID, 'user', 'other')
    store.addLLMDetails(a.ID, 'model', 1, '{}')

    store.deleteMessagesFrom(a.ID, 2)
    expect(store.getMessages(a.ID).map(m => m.Content)).toEqual(['q1', 'a1'])
    store.deleteMessagesFrom(a.ID, 10)
    expect(store.getMessages(a.ID)).toHaveLength(2)

    store.deleteConversation(a.ID)
    expect(store.getMessages(a.ID)).toEqual([])
    expect(store.llmDetails).toEqual([])
    expect(store.getMessages(b.ID)).toHaveLength(1)
  })

  it('lists the most recently updated chat first and searches case-insensitively', () => {
    const store = new FakeStore()
    const older = store.createConversation()
    const newer = store.createConversation()
    store.addMessage(newer.ID, 'user', 'Sourdough')
    store.addMessage(older.ID, 'user', 'Kubernetes Ingress')
    expect(store.listConversations().map(c => c.ID)).toEqual([older.ID, newer.ID])
    expect(Object.keys(store.searchMessages('ingress')).map(Number)).toEqual([older.ID])
  })
})

describe('createFakeBackend', () => {
  it.each(fingerprintCases.map(c => [c.name, c] as const))('fingerprints %s like the Go backend', (_name, c) => {
    expect(connectionSettingsFingerprint(c.settings)).toBe(c.fingerprint)
  })

  it('streams a scripted reply and stores the turn', async () => {
    const fake = createFakeBackend({ replies: [{ text: 'Hello there!', chunkSize: 5, chunkDelayMs: 0, firstTokenDelayMs: 0 }] })
    const chunks: string[] = []
    fake.events.on('stream_chunk', (_id: unknown, chunk: unknown) => chunks.push(String(chunk)), -1)

    const conv = await fake.app.CreateConversation()
    const reply = await fake.app.SendMessage(conv.ID, 'Hi', '')

    expect(reply).toBe('Hello there!')
    expect(chunks.join('')).toBe('Hello there!')
    const stored = await fake.app.GetMessages(conv.ID)
    expect(stored.map(m => [m.Role, m.Content])).toEqual([
      ['user', 'Hi'],
      ['assistant', 'Hello there!'],
    ])
    expect(fake.control.calls('SendMessage')).toHaveLength(1)
  })

  it('reports errors the way the Go bindings do', async () => {
    const fake = createFakeBackend()
    await expect(fake.app.DeleteMessagesFrom(1, -1)).rejects.toBe('invalid message index -1')
    expect(await fake.app.SendMessage(404, 'Anyone?', '')).toMatch(/^Error: Failed to save message: FOREIGN KEY/)
  })
})
