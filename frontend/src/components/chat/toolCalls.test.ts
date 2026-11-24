import { expect, it } from 'vitest'
import { summarizeTools, toolCallArgDisplay, toolMeta } from './toolCalls'

it('labels known tools and humanizes unknown ones', () => {
  expect(toolMeta('webclaw').label).toBe('Read web page')
  expect(toolMeta('my_custom_tool').label).toBe('my custom tool')
})

it('shows the argument that best describes a call', () => {
  expect(toolCallArgDisplay({ name: 'web_search', argsJSON: '{"query":"wails v3","limit":5}' })).toBe('“wails v3”')
  expect(toolCallArgDisplay({ name: 'read_file', argsJSON: '{"offset":10,"path":"/src/app.go"}' })).toBe('/src/app.go')
  expect(toolCallArgDisplay({ name: 'grep', argsJSON: '{"query":"  ","pattern":"TODO"}' })).toBe('TODO')
  expect(toolCallArgDisplay({ name: 'tool_search', argsJSON: '{"limit":3}' })).toBe('')
  // Arguments arrive incrementally while streaming.
  expect(toolCallArgDisplay({ name: 'web_search', argsJSON: '{"query":"wai' })).toBe('')
})

it('summarizes distinct tool labels in first-use order', () => {
  const calls = ['tool_search', 'web_search', 'webclaw', 'websearch', 'web_search'].map(name => ({ name, argsJSON: '{}' }))
  expect(summarizeTools(calls)).toEqual(['Discover tools', 'Web search', 'Read web page'])
})
