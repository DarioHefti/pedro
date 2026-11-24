import type { ReplyScript } from './types'

/**
 * Canned model replies shaped like real Pedro conversations: tool discovery, web research with
 * sources, mermaid diagrams, code, and file exploration. No real LLM is ever called.
 */

const PATH_RE = /\[Path: ([^\]]+)\]/

function topicOf(question: string): string {
  const firstLine = question.split('\n')[0].trim()
  return firstLine.length > 80 ? `${firstLine.slice(0, 77)}…` : firstLine || 'your question'
}

export const RESEARCH_REPLY = (question: string): ReplyScript => ({
  toolCalls: [
    { name: 'tool_search', args: { query: 'web_search', mode: 'regex' }, result: '{"type":"tool_reference","tool_references":[{"tool_name":"web_search"}]}' },
    { name: 'web_search', args: { query: topicOf(question) }, result: 'Title: Release notes\nURL: https://example.com/release-notes\nSnippet: The latest release adds streaming tool calls.' },
    { name: 'webclaw', args: { url: 'https://example.com/release-notes' }, result: '# Release notes\n\nStreaming tool calls, faster cold starts.' },
  ],
  text: [
    `Here's a quick summary of **${topicOf(question)}**:`,
    '',
    '## Highlights',
    '',
    '- Streaming tool calls are now supported end to end',
    '- Cold starts are roughly 30% faster',
    '- The configuration format is unchanged',
    '',
    '| Area | Change |',
    '| --- | --- |',
    '| Runtime | Faster startup |',
    '| API | Streaming tool calls |',
    '',
    'Sources: [Release notes](https://example.com/release-notes)',
  ].join('\n'),
})

export const DIAGRAM_REPLY: ReplyScript = {
  text: [
    'Here is the sign-in flow as a diagram:',
    '',
    '```mermaid',
    'flowchart TD',
    '    A[User opens Pedro] --> B{Signed in?}',
    '    B -- yes --> C[Start chatting]',
    '    B -- no --> D[Sign in with Azure]',
    '    D --> C',
    '```',
    '',
    'Each step maps to one screen in the app.',
  ].join('\n'),
}

export const CODE_REPLY: ReplyScript = {
  text: [
    'A minimal HTTP server in Go:',
    '',
    '```go',
    'package main',
    '',
    'import "net/http"',
    '',
    'func main() {',
    '\thttp.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {',
    '\t\tw.Write([]byte("hello"))',
    '\t})',
    '\thttp.ListenAndServe(":8080", nil)',
    '}',
    '```',
    '',
    'Run it with `go run main.go`.',
  ].join('\n'),
}

export const FILE_REPLY = (path: string, folder: boolean): ReplyScript => ({
  toolCalls: folder
    ? [{ name: 'show_file_tree', args: { path, depth: 2 }, result: `${path}\n├── README.md\n└── src/` }]
    : [{ name: 'read_file', args: { path, offset: 1, limit: 120 }, result: '1: # Notes\n2: Ship the redesign on Friday.' }],
  text: folder
    ? `The project at \`${path}\` has a README and a \`src\` folder. Start with the README.`
    : `The file \`${path}\` contains short notes. The key point: **ship the redesign on Friday**.`,
})

export const GENERIC_REPLY = (question: string): ReplyScript => ({
  text: [
    `Sure! Here's what I can tell you about “${topicOf(question)}”:`,
    '',
    '1. Start small and iterate.',
    '2. Keep things **simple** and *readable*.',
    '3. Use `inline code` for exact names.',
  ].join('\n'),
})

/** Picks a canned reply from what the person asked, like a (very) small model would. */
export function defaultReply(question: string): ReplyScript {
  const q = question.toLowerCase()
  const path = PATH_RE.exec(question)?.[1]
  if (path) return FILE_REPLY(path, question.includes('[Folder:'))
  if (/\b(diagram|mermaid|flowchart)\b/.test(q)) return DIAGRAM_REPLY
  if (/\b(code|function|snippet|server)\b/.test(q)) return CODE_REPLY
  if (/\b(research|search|latest|news)\b/.test(q)) return RESEARCH_REPLY(question)
  return GENERIC_REPLY(question)
}

/** Splits text into stream chunks of at most `size` characters. */
export function chunkText(text: string, size: number): string[] {
  const chunks: string[] = []
  for (let i = 0; i < text.length; i += size) chunks.push(text.slice(i, i + size))
  return chunks
}
