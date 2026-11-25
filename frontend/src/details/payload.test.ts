import { describe, expect, it } from 'vitest'
import type { LLMDetailsEntry } from '../services/wailsService'
import { buildRow, compactNumber, groupByDay, humanSize, normalizeContent, parsePayload, prettyJson } from './payload'

const captured = {
  request: {
    model: 'gpt-4o',
    temperature: 0.2,
    stream: true,
    tool_choice: 'auto',
    tools: [{ type: 'function', function: { name: 'web_search' } }],
    messages: [
      { role: 'system', content: 'You are Pedro.' },
      { role: 'user', content: 'First question' },
      { role: 'assistant', content: null, tool_calls: [{ id: 'c1', type: 'function', function: { name: 'web_search', arguments: '{}' } }] },
      { role: 'tool', content: 'results', tool_call_id: 'c1' },
      {
        role: 'user',
        content: [
          { type: 'text', text: 'What is in   this\nimage?' },
          { type: 'image_url', image_url: { url: 'data:image/png;base64,AA' } },
        ],
      },
    ],
  },
  response: 'A cat.',
}

const entry = (CreatedAt: string, payload: unknown = captured): LLMDetailsEntry =>
  ({ ID: 1, ConversationID: 1, ConversationTitle: 'Cats', CreatedAt, RequestCount: 1, Model: 'gpt-4o', Messages: JSON.stringify(payload) }) as LLMDetailsEntry

describe('parsePayload', () => {
  it('reads the captured request, its sampling params and the response', () => {
    const p = parsePayload(JSON.stringify(captured))
    expect(p.model).toBe('gpt-4o')
    expect(p.messages).toHaveLength(5)
    expect(p.tools).toHaveLength(1)
    expect(p.params).toEqual({ temperature: 0.2, tool_choice: 'auto' })
    expect(p.response).toBe('A cat.')
    expect(p.roleCounts).toEqual({ system: 1, user: 2, assistant: 1, tool: 1 })
  })

  it('returns an empty payload for malformed or request-less JSON', () => {
    for (const raw of ['{oops', '{}', '{"request":"text"}']) {
      const p = parsePayload(raw)
      expect(p.messages).toEqual([])
      expect(p.model).toBe('')
    }
  })
})

it('normalizes every OpenAI content shape to text', () => {
  expect(normalizeContent(null)).toBe('')
  expect(normalizeContent('plain')).toBe('plain')
  expect(normalizeContent([{ type: 'text', text: 'a' }, { type: 'image_url', image_url: { url: 'u' } }])).toBe('a\n[image: u]')
})

it('previews the last user message and indexes the row for search', () => {
  const row = buildRow(entry('2026-11-24T08:00:00Z'))
  expect(row.preview).toBe('What is in this image? [image: data:image/png;base64,AA]')
  expect(row.searchText).toContain('cats')
  expect(row.searchText).toContain('first question')
  expect(row.createdAt?.toISOString()).toBe('2026-11-24T08:00:00.000Z')
})

it('groups consecutive rows by calendar day', () => {
  const now = new Date(2026, 10, 24, 18)
  const rows = [
    new Date(2026, 10, 24, 9),
    new Date(2026, 10, 24, 8),
    new Date(2026, 10, 23, 22),
    new Date(2026, 10, 20, 12),
  ].map(d => buildRow(entry(d.toISOString())))
  rows.push(buildRow(entry('')))
  const groups = groupByDay(rows, now)
  expect(groups.map(g => [g.label, g.rows.length])).toEqual([
    ['Today', 2],
    ['Yesterday', 1],
    [expect.any(String), 1],
    ['Unknown date', 1],
  ])
})

it('formats sizes and counts compactly', () => {
  expect(humanSize(999)).toBe('999 chars')
  expect(humanSize(1500)).toBe('1.5k chars')
  expect(humanSize(25_000)).toBe('25k chars')
  expect(humanSize(2_500_000)).toBe('2.5M chars')
  expect(compactNumber(999)).toBe('999')
  expect(compactNumber(1234)).toBe('1.2k')
  expect(compactNumber(12_345)).toBe('12k')
  expect(compactNumber(1_500_000)).toBe('1.5M')
  expect(prettyJson('{"a":1}')).toBe('{\n  "a": 1\n}')
  expect(prettyJson('not json')).toBe('not json')
})
