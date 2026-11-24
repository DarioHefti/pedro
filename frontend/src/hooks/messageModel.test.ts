import { describe, expect, it } from 'vitest'
import type { Message } from '../services/wailsService'
import {
  appendUserMessage,
  buildLLMContent,
  buildThreadState,
  emptyThread,
  isErrorResponse,
  stripFileAnnotations,
  truncateThread,
  type Attachment,
} from './messageModel'

const msg = (m: Partial<Message>): Message => ({ Role: 'user', Content: '', Attachments: '', ToolCalls: '', ...m }) as Message

const fileRef = (path: string): Attachment => ({ type: 'file-ref', content: path, name: path.split('/').pop()! })
const folderRef = (path: string): Attachment => ({ type: 'folder-ref', content: path, name: path.split('/').pop()! })
const image: Attachment = { type: 'image', content: 'data:image/png;base64,AAAA', name: 'shot.png' }

describe('buildLLMContent', () => {
  it('leaves plain messages and image-only messages unchanged', () => {
    expect(buildLLMContent('Hello')).toBe('Hello')
    expect(buildLLMContent('Look', [image])).toBe('Look')
  })

  it('tells the model how to read each kind of attachment', () => {
    const content = buildLLMContent('Summarize', [
      fileRef('/docs/report.pdf'),
      fileRef('/pics/chart.png'),
      fileRef('/audio/call.m4a'),
      folderRef('/src/app'),
      { type: 'text', content: 'line 1', name: 'notes.txt' },
    ])
    const [typed, ...notes] = content.split('\n\n')
    expect(typed).toBe('Summarize')
    expect(notes).toHaveLength(5)
    expect(notes[0]).toMatch(/^\[File: report\.pdf\]\n\[Path: \/docs\/report\.pdf\]\n.*parse_document/)
    expect(notes[1]).toContain('Raster image attached by path')
    expect(notes[2]).toContain('transcribe_audio')
    expect(notes[3]).toMatch(/^\[Folder: app\]\n\[Path: \/src\/app\]\n.*show_file_tree/)
    expect(notes[4]).toBe('[File: notes.txt]\nline 1')
  })

  it('round-trips through stripFileAnnotations', () => {
    const sent = buildLLMContent('What is in here?', [fileRef('/a/b.go'), folderRef('/a')])
    expect(stripFileAnnotations(sent)).toBe('What is in here?')
    expect(stripFileAnnotations('No notes\n\nJust paragraphs')).toBe('No notes\n\nJust paragraphs')
  })
})

describe('buildThreadState', () => {
  it('decodes attachments and tool calls by message index', () => {
    const thread = buildThreadState([
      msg({ Content: 'q', Attachments: JSON.stringify([image, fileRef('/a/b.go'), folderRef('/a')]) }),
      msg({ Role: 'assistant', ToolCalls: JSON.stringify([{ name: 'grep', argsJSON: '{"pattern":"x"}', id: 'c1' }]) }),
      msg({ Role: 'tool', Content: 'match' }),
      msg({ Role: 'assistant', Content: 'done' }),
    ])
    expect(thread.messages).toHaveLength(4)
    expect(thread.images.get(0)).toEqual([image.content])
    expect(thread.files.get(0)).toEqual([
      { name: 'b.go', path: '/a/b.go', type: 'file' },
      { name: 'a', path: '/a', type: 'folder' },
    ])
    expect(thread.toolCalls.get(1)).toEqual([{ name: 'grep', argsJSON: '{"pattern":"x"}', id: 'c1' }])
    expect([...thread.toolCalls.keys()]).toEqual([1])
  })

  it('ignores malformed or partial JSON instead of failing the whole thread', () => {
    const thread = buildThreadState([
      msg({ Attachments: '{not json' }),
      msg({ Role: 'assistant', ToolCalls: '{"name":"grep"}' }),
      msg({ Role: 'assistant', ToolCalls: JSON.stringify([{ name: 'grep' }, { name: 'glob', argsJSON: '{}' }]) }),
    ])
    expect(thread.images.size + thread.files.size).toBe(0)
    expect(thread.toolCalls.get(1)).toBeUndefined()
    expect(thread.toolCalls.get(2)).toEqual([{ name: 'glob', argsJSON: '{}' }])
  })
})

describe('truncateThread', () => {
  const thread = buildThreadState([
    msg({ Content: 'q1', Attachments: JSON.stringify([image]) }),
    msg({ Role: 'assistant', Content: 'a1' }),
    msg({ Content: 'q2', Attachments: JSON.stringify([fileRef('/x.txt')]) }),
    msg({ Role: 'assistant', ToolCalls: JSON.stringify([{ name: 'grep', argsJSON: '{}' }]) }),
  ])

  it('keeps messages up to and including the index, with their extras', () => {
    const cut = truncateThread(thread, 1)
    expect(cut.messages.map(m => m.Content)).toEqual(['q1', 'a1'])
    expect([...cut.images.keys()]).toEqual([0])
    expect(cut.files.size).toBe(0)
    expect(cut.toolCalls.size).toBe(0)
  })

  it('empties the thread for a negative index and leaves the input untouched', () => {
    expect(truncateThread(thread, -1)).toEqual(emptyThread())
    expect(thread.messages).toHaveLength(4)
  })
})

describe('appendUserMessage', () => {
  it('adds the message and its extras at the next index without mutating the thread', () => {
    const before = buildThreadState([msg({ Content: 'q1' }), msg({ Role: 'assistant', Content: 'a1' })])
    const after = appendUserMessage(before, 'q2', [image, fileRef('/a/b.go')])
    expect(after.messages.map(m => [m.Role, m.Content])).toEqual([
      ['user', 'q1'],
      ['assistant', 'a1'],
      ['user', 'q2'],
    ])
    expect(after.images.get(2)).toEqual([image.content])
    expect(after.files.get(2)).toEqual([{ name: 'b.go', path: '/a/b.go', type: 'file' }])
    expect(before.messages).toHaveLength(2)
    expect(before.images.size).toBe(0)
  })
})

it('recognizes backend error replies', () => {
  expect(isErrorResponse('Error: rate limited')).toBe(true)
  expect(isErrorResponse('An Error: inside text')).toBe(false)
  expect(isErrorResponse(undefined)).toBe(false)
})
