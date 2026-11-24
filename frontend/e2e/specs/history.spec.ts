import { expect, test, type FakeSeed } from '../fixtures'
import { RESEARCH_REPLY } from '../fake-backend/replies'
import { conversation, exchange } from '../fake-backend/seeds'

const TWO_TURNS: FakeSeed = {
  conversations: [
    {
      title: 'Planning chat',
      daysAgo: 0,
      requestCount: 3,
      requestTokens: 1200,
      messages: [
        { role: 'user', content: 'First question' },
        { role: 'assistant', content: 'First answer' },
        { role: 'user', content: 'Second question' },
        { role: 'assistant', content: 'Second answer' },
      ],
    },
    { title: 'Other chat', daysAgo: 1, messages: [{ role: 'user', content: 'Other question' }, { role: 'assistant', content: 'Other answer' }] },
  ],
}

test.describe('conversation history', () => {
  test('opening a chat loads its thread and stats', async ({ pedro, page }) => {
    await pedro.open(TWO_TURNS)
    await pedro.chat('Planning chat').click()

    await expect(pedro.chatTitle).toHaveText('Planning chat')
    await expect(pedro.userBubbles).toHaveText(['First question', 'Second question'])
    await expect(pedro.replyBubbles).toHaveText([/First answer/, /Second answer/])
    await expect(page.getByRole('button', { name: '3 requests' })).toBeVisible()
    await expect(page.getByRole('button', { name: '1.2K tokens' })).toBeVisible()
    await expect(pedro.composer).toBeFocused()

    await pedro.chat('Other chat').click()
    await expect(pedro.chatTitle).toHaveText('Other chat')
    await expect(pedro.userBubbles).toHaveText(['Other question'])
    await expect(page.getByRole('button', { name: '0 requests' })).toBeVisible()
  })

  test('stored tool calls show on the reply that used them', async ({ pedro, page }) => {
    await pedro.open({ conversations: [conversation('Research chat', 0, exchange('Research Wails', RESEARCH_REPLY('Research Wails')))] })
    await pedro.chat('Research chat').click()
    await expect(page.getByRole('button', { name: /Used 3 tools/ })).toBeVisible()
    await expect(pedro.replies.last()).toContainText('Highlights')
  })

  test('regenerate replaces a reply', async ({ pedro }) => {
    await pedro.open(TWO_TURNS)
    await pedro.chat('Planning chat').click()
    await pedro.queueReply({ text: 'A brand new second answer' })

    await pedro.replies.last().getByRole('button', { name: 'Regenerate' }).click()
    await pedro.waitForIdle()

    await expect(pedro.replyBubbles).toHaveText([/First answer/, /A brand new second answer/])
    const calls = await pedro.calls('RegenerateMessage')
    expect(calls.map(c => c.args[1])).toEqual([3])
  })

  test('regenerating an earlier reply drops the turns after it', async ({ pedro }) => {
    await pedro.open(TWO_TURNS)
    await pedro.chat('Planning chat').click()
    await pedro.queueReply({ text: 'Rewritten first answer' })

    await pedro.replies.first().getByRole('button', { name: 'Regenerate' }).click()
    await pedro.waitForIdle()

    await expect(pedro.userBubbles).toHaveText(['First question'])
    await expect(pedro.replyBubbles).toHaveText([/Rewritten first answer/])
  })

  test('resend answers a question again and drops what followed', async ({ pedro }) => {
    await pedro.open(TWO_TURNS)
    await pedro.chat('Planning chat').click()
    await pedro.queueReply({ text: 'Another take on the first question' })

    await pedro.userMessages.first().getByRole('button', { name: 'Resend' }).click()
    await pedro.waitForIdle()

    await expect(pedro.userBubbles).toHaveText(['First question'])
    await expect(pedro.replyBubbles).toHaveText([/Another take on the first question/])
    expect((await pedro.calls('ResendMessage')).map(c => c.args[1])).toEqual([0])
  })

  test('delete from here removes that message and everything after it', async ({ pedro }) => {
    await pedro.open(TWO_TURNS)
    await pedro.chat('Planning chat').click()

    await pedro.userMessages.last().getByRole('button', { name: 'Delete from here' }).click()

    await expect(pedro.userBubbles).toHaveText(['First question'])
    await expect(pedro.replyBubbles).toHaveText([/First answer/])
    const snap = await pedro.snapshot()
    expect(snap.messages.filter(m => m.ConversationID === snap.conversations.find(c => c.Title === 'Planning chat')!.ID)).toHaveLength(2)
  })

  test('deleting from the first message removes the chat', async ({ pedro, page }) => {
    await pedro.open(TWO_TURNS)
    await pedro.chat('Planning chat').click()

    await pedro.userMessages.first().getByRole('button', { name: 'Delete from here' }).click()

    await expect(pedro.chat('Planning chat')).toHaveCount(0)
    await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Pedro' })).toBeVisible()
    await expect(pedro.chat('Other chat')).toBeVisible()
  })

  test('after a chat is emptied, the next message starts a new chat', async ({ pedro }) => {
    await pedro.open(TWO_TURNS)
    await pedro.chat('Planning chat').click()
    await pedro.userMessages.first().getByRole('button', { name: 'Delete from here' }).click()
    await expect(pedro.chat('Planning chat')).toHaveCount(0)

    await pedro.ask('A fresh start')

    await expect(pedro.chat('A fresh start')).toBeVisible()
    await expect(pedro.chatTitle).toHaveText('A fresh start')
    await expect(pedro.toast(/Failed to save message/)).toHaveCount(0)
    expect(await pedro.calls('CreateConversation')).toHaveLength(1)
  })

  test('the thread follows new content until the reader scrolls up', async ({ pedro, page }) => {
    const long = Array.from({ length: 12 }, (_, i) => [
      { role: 'user' as const, content: `Question ${i + 1}` },
      { role: 'assistant' as const, content: `Answer ${i + 1}\n\n${'A long paragraph of text. '.repeat(30)}` },
    ]).flat()
    await pedro.open({ conversations: [{ title: 'Long chat', daysAgo: 0, messages: long }] })
    await pedro.chat('Long chat').click()

    const thread = page.locator('.thread')
    const distanceFromBottom = () => thread.evaluate(el => el.scrollHeight - el.scrollTop - el.clientHeight)
    await expect(pedro.replyBubbles.last()).toBeInViewport()
    await expect.poll(distanceFromBottom).toBeLessThan(80)

    await thread.evaluate(el => el.scrollTo({ top: 0 }))
    const jump = page.getByRole('button', { name: 'Jump to latest' })
    await expect(jump).toBeVisible()

    await jump.click()
    await expect(jump).toHaveCount(0)
    await expect(pedro.replyBubbles.last()).toBeInViewport()
  })

  test('a reply keeps streaming in the background while another chat is open', async ({ pedro }) => {
    await pedro.open(TWO_TURNS)
    await pedro.queueReply({ text: 'Background answer that takes a while to arrive.', chunkSize: 4, chunkDelayMs: 80 })
    await pedro.send('Start a background reply')
    await expect(pedro.streamingReply).toBeVisible()

    await pedro.chat('Other chat').click()
    await expect(pedro.userBubbles).toHaveText(['Other question'])
    await expect(pedro.streamingReply).toHaveCount(0)
    // Any running reply blocks sending, in every chat.
    await expect(pedro.stopButton).toBeVisible()

    await expect(pedro.sendButton).toBeVisible({ timeout: 15_000 })
    await expect(pedro.userBubbles).toHaveText(['Other question'])

    await pedro.chat('Start a background reply').click()
    await expect(pedro.replies.last()).toContainText('Background answer that takes a while to arrive.')
  })
})
