import { expect, test } from '../fixtures'
import { CODE_REPLY, DIAGRAM_REPLY } from '../fake-backend/replies'

const SLOW_REPLY = {
  text: 'This answer streams slowly so there is time to look at it while it is still being written.',
  chunkSize: 4,
  chunkDelayMs: 120,
}

test.describe('chatting', () => {
  test('sending streams a reply and creates the chat', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.queueReply({ ...SLOW_REPLY, text: 'Hello! **Nice** to meet you.' })

    await pedro.send('Hi Pedro, who are you?')

    await expect(pedro.userMessages).toHaveCount(1)
    await expect(pedro.userMessages.first()).toContainText('Hi Pedro, who are you?')
    await expect(pedro.composer).toHaveValue('')
    await expect(pedro.streamingReply).toBeVisible()
    await expect(pedro.stopButton).toBeVisible()

    await pedro.waitForIdle()
    const reply = pedro.replies.last()
    await expect(reply).toContainText('Hello! Nice to meet you.')
    await expect(reply.locator('[data-streamdown="strong"]')).toHaveText('Nice')

    await expect(pedro.chat('Hi Pedro, who are you?')).toBeVisible()
    await expect(pedro.chat('Hi Pedro, who are you?')).toHaveAttribute('aria-current', 'page')
    await expect(pedro.chatTitle).toHaveText('Hi Pedro, who are you?')
    await expect(page.getByRole('button', { name: '1 requests' })).toBeVisible()
    await expect(page.getByRole('button', { name: '240 tokens' })).toBeVisible()

    const sends = await pedro.calls('SendMessage')
    expect(sends).toHaveLength(1)
    expect(sends[0].args[1]).toBe('Hi Pedro, who are you?')
  })

  test('the send button follows the composer content', async ({ pedro }) => {
    await pedro.open()
    await expect(pedro.sendButton).toBeDisabled()
    await pedro.composer.fill('   ')
    await expect(pedro.sendButton).toBeDisabled()
    await pedro.composer.fill('Something to say')
    await expect(pedro.sendButton).toBeEnabled()
    await pedro.sendButton.click()
    await expect(pedro.userMessages).toHaveCount(1)
    await pedro.waitForIdle()
  })

  test('shift+enter adds a line instead of sending', async ({ pedro }) => {
    await pedro.open()
    await pedro.composer.click()
    await pedro.page.keyboard.type('first line')
    await pedro.page.keyboard.press('Shift+Enter')
    await pedro.page.keyboard.type('second line')
    await expect(pedro.composer).toHaveValue('first line\nsecond line')
    await expect(pedro.userMessages).toHaveCount(0)
    expect(await pedro.calls('SendMessage')).toHaveLength(0)
  })

  test('a second message continues the same chat', async ({ pedro }) => {
    await pedro.open()
    await pedro.ask('First question')
    await pedro.ask('Second question')
    await expect(pedro.userMessages).toHaveCount(2)
    await expect(pedro.replies).toHaveCount(2)
    await expect(pedro.chatList.getByRole('listitem')).toHaveCount(1)
    expect(await pedro.calls('CreateConversation')).toHaveLength(1)
  })

  test('tool calls are summarized and can be expanded', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.ask('Please research the latest Wails release')

    const group = page.getByRole('button', { name: /Used 3 tools/ })
    await expect(group).toBeVisible()
    await expect(group).toContainText('Discover tools')
    await expect(group).toContainText('Web search')
    await expect(group).toContainText('Read web page')

    await group.click()
    const steps = page.locator('.tool-steps .tool-step')
    await expect(steps).toHaveCount(3)
    await expect(steps.nth(1)).toContainText('“Please research the latest Wails release”')
    await expect(steps.nth(2)).toContainText('https://example.com/release-notes')

    await expect(page.getByRole('button', { name: '2 requests' })).toBeVisible()
  })

  test('tool calls show live while the reply is running', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.queueReply({
      toolCalls: [{ name: 'web_search', args: { query: 'pedro release' } }],
      text: SLOW_REPLY.text,
      chunkDelayMs: 150,
      chunkSize: 3,
    })
    await pedro.send('Search for the release')
    await expect(pedro.streamingReply.getByText(/Working 1 tool/)).toBeVisible()
    await expect(pedro.streamingReply.locator('.tool-step')).toContainText('“pedro release”')
    await pedro.waitForIdle()
    await expect(page.getByRole('button', { name: /Used 1 tool/ })).toBeVisible()
  })

  test('markdown renders headings, lists, tables and links', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.ask('Please research the latest Wails release')
    const reply = pedro.replies.last()

    await expect(reply.getByRole('heading', { name: 'Highlights' })).toBeVisible()
    await expect(reply.getByRole('listitem')).toHaveCount(3)
    await expect(reply.getByRole('table')).toBeVisible()
    await expect(reply.getByRole('row')).toHaveCount(3)

    const link = reply.getByRole('link', { name: 'Release notes' })
    await expect(link).toHaveAttribute('href', 'https://example.com/release-notes')
    const pagesBefore = page.context().pages().length
    await link.click()
    await expect.poll(async () => (await pedro.calls('BrowserOpenURL')).map(c => c.args[0])).toEqual([
      'https://example.com/release-notes',
    ])
    expect(page.context().pages()).toHaveLength(pagesBefore)
  })

  test('copying a reply puts its markdown on the clipboard', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.queueReply({ text: 'Copy **me** please' })
    await pedro.ask('Give me something to copy')

    await pedro.replies.last().getByRole('button', { name: 'Copy' }).click()
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe('Copy **me** please')
  })

  test('a code block can be saved through the native dialog', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.queueReply(CODE_REPLY)
    await pedro.ask('Show me code for a tiny Go HTTP server')

    const block = pedro.replies.last().locator('[data-streamdown="code-block"]')
    await expect(block).toContainText('package main')
    await block.locator('[data-streamdown="code-block-download-button"]').click()

    await expect(pedro.toast('File saved')).toBeVisible()
    const saves = await pedro.calls('SaveFile')
    expect(saves).toHaveLength(1)
    expect(saves[0].args[0]).toBe('download.go')
    expect(String(saves[0].args[1])).toContain('http.ListenAndServe(":8080", nil)')
    await expect(page.getByRole('link', { name: /download/ })).toHaveCount(0)
  })

  test('a mermaid block renders as a diagram', async ({ pedro }) => {
    await pedro.open()
    await pedro.queueReply(DIAGRAM_REPLY)
    await pedro.ask('Draw the sign-in flow')

    const diagram = pedro.replies.last().locator('.diagram-block')
    await expect(diagram).toHaveAttribute('data-state', 'ready', { timeout: 20_000 })
    await expect(diagram.locator('.diagram-preview-svg svg')).toBeVisible()
    await expect(diagram.locator('.diagram-preview-svg')).toContainText('Sign in with Azure')

    const heightWithDiagram = (await diagram.boundingBox())?.height
    await diagram.getByRole('button', { name: 'Code', exact: true }).click()
    await expect(diagram.locator('.diagram-code')).toContainText('flowchart TD')
    // Toggling keeps the block's height, so the thread doesn't jump.
    expect((await diagram.boundingBox())?.height).toBeCloseTo(heightWithDiagram ?? 0, 0)
    await diagram.getByRole('button', { name: 'Diagram', exact: true }).click()
    await expect(diagram.locator('.diagram-preview-svg svg')).toBeVisible()
    expect((await diagram.boundingBox())?.height).toBeCloseTo(heightWithDiagram ?? 0, 0)
  })

  test('a provider error is reported and the question is kept', async ({ pedro }) => {
    await pedro.open()
    await pedro.queueReply({ error: '429 rate limit exceeded' })
    await pedro.send('Will this fail?')

    await expect(pedro.toast('Error: 429 rate limit exceeded')).toBeVisible()
    await pedro.waitForIdle()
    await expect(pedro.userMessages).toHaveCount(1)
    await expect(pedro.replies).toHaveCount(0)
    await expect(pedro.chat('Will this fail?')).toBeVisible()
  })

  test('without a provider the app asks to configure one', async ({ pedro }) => {
    await pedro.open({ unconfigured: true })
    await pedro.send('Hello?')
    await expect(pedro.toast(/please configure LLM provider settings first/)).toBeVisible()
    await expect(pedro.userMessages).toHaveCount(1)
    await expect(pedro.replies).toHaveCount(0)
  })

  test('stop ends a reply early and keeps what was written so far', async ({ pedro }) => {
    await pedro.open()
    await pedro.queueReply({ ...SLOW_REPLY, chunkDelayMs: 200 })
    await pedro.send('Tell me a long story')

    await expect(pedro.streamingReply).toContainText('This')
    await pedro.stopButton.click()

    await pedro.waitForIdle()
    await expect(pedro.userMessages).toHaveCount(1)
    await expect(pedro.replies).toHaveCount(1)
    await expect(pedro.replyBubbles).toContainText('This')
    await expect(pedro.replyBubbles).not.toContainText('still being written')
    expect(await pedro.calls('AbortMessage')).toHaveLength(1)

    // The partial reply is stored: it is still there after opening the chat again.
    await pedro.shortcut('n')
    await expect(pedro.replies).toHaveCount(0)
    await pedro.chat('Tell me a long story').click()
    await expect(pedro.replyBubbles).toContainText('This')
    await pedro.ask('Are you still there?')
    await expect(pedro.replies).toHaveCount(2)
  })

  test('stopping during a tool call keeps the tools that ran', async ({ pedro }) => {
    await pedro.open()
    await pedro.queueReply({
      toolCalls: [{ name: 'web_search', args: { query: 'pedro' }, result: 'results' }],
      text: 'Never streamed.',
      chunkDelayMs: 1000,
    })
    await pedro.send('Search for pedro')
    await expect(pedro.stopButton).toBeVisible()
    await expect(pedro.streamingReply.getByText(/Working 1 tool/)).toBeVisible()
    await pedro.stopButton.click()

    await pedro.waitForIdle()
    await expect(pedro.replies).toHaveCount(1)
    await expect(pedro.replyBubbles).toHaveCount(0)
  })

  test('escape stops a running reply', async ({ pedro }) => {
    await pedro.open()
    await pedro.queueReply({ ...SLOW_REPLY, chunkDelayMs: 200 })
    await pedro.send('Another long story')
    await expect(pedro.streamingReply).toContainText('This')
    await pedro.composer.press('Escape')
    await pedro.waitForIdle()
    expect(await pedro.calls('AbortMessage')).toHaveLength(1)
  })

  test('a second message cannot be sent while a reply runs', async ({ pedro }) => {
    await pedro.open()
    await pedro.queueReply(SLOW_REPLY)
    await pedro.send('First')
    await expect(pedro.stopButton).toBeVisible()

    await pedro.composer.fill('Second')
    await pedro.composer.press('Enter')
    await expect(pedro.composer).toHaveValue('Second')
    expect(await pedro.calls('SendMessage')).toHaveLength(1)

    await pedro.waitForIdle()
    await pedro.composer.press('Enter')
    await expect(pedro.userMessages).toHaveCount(2)
    await pedro.waitForIdle()
  })
})
