import { expect, test, type FakeSeed } from '../fixtures'

const reply = (text: string) => [
  { role: 'user' as const, content: text },
  { role: 'assistant' as const, content: `About ${text}` },
]

const HISTORY: FakeSeed = {
  conversations: [
    { title: 'Today chat', daysAgo: 0, messages: reply('kubernetes ingress') },
    { title: 'Yesterday chat', daysAgo: 1, messages: reply('sourdough starter') },
    { title: 'This week chat', daysAgo: 3, messages: reply('swiss train passes') },
    { title: 'This month chat', daysAgo: 12, messages: reply('tax deadlines') },
    { title: 'Ancient chat', daysAgo: 90, messages: reply('old laptop') },
  ],
}

test.describe('sidebar', () => {
  test('chats are grouped by recency, newest first', async ({ pedro }) => {
    await pedro.open(HISTORY)
    const groups = pedro.chatList.getByRole('region')
    await expect(groups).toHaveCount(5)
    await expect(groups.getByRole('heading')).toHaveText(['Today', 'Yesterday', 'Previous 7 days', 'Previous 30 days', 'Older'])
    await expect(pedro.chatList.locator('.conv-row-title')).toHaveText([
      'Today chat',
      'Yesterday chat',
      'This week chat',
      'This month chat',
      'Ancient chat',
    ])
    await expect(pedro.sidebar.locator('.sidebar-section-title')).toHaveText('Chats5')
  })

  test('search matches titles and message content', async ({ pedro }) => {
    await pedro.open(HISTORY)
    const search = pedro.sidebar.getByRole('textbox', { name: 'Search chats' })

    await search.fill('yesterday')
    await expect(pedro.chatList.getByRole('listitem')).toHaveCount(1)
    await expect(pedro.chat('Yesterday chat')).toBeVisible()
    await expect(pedro.sidebar.locator('.sidebar-section-title')).toHaveText('Results1')

    await search.fill('train passes')
    await expect(pedro.chat('This week chat')).toBeVisible()
    await expect(pedro.chatList.getByRole('listitem')).toHaveCount(1)
    expect((await pedro.calls('SearchMessages')).map(c => c.args[0])).toContain('train passes')

    await search.fill('nothing like this')
    await expect(pedro.sidebar.getByText('No matches')).toBeVisible()

    await pedro.sidebar.getByRole('button', { name: 'Clear search' }).click()
    await expect(pedro.chatList.getByRole('listitem')).toHaveCount(5)
  })

  test('a chat can be deleted from its row', async ({ pedro, page }) => {
    await pedro.open(HISTORY)
    await pedro.chat('Yesterday chat').click()
    await expect(pedro.chatTitle).toHaveText('Yesterday chat')

    await pedro.chat('Yesterday chat').hover()
    await pedro.chatList.getByRole('button', { name: 'Delete Yesterday chat' }).click()

    await expect(pedro.chat('Yesterday chat')).toHaveCount(0)
    await expect(pedro.chatList.getByRole('listitem')).toHaveCount(4)
    await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Pedro' })).toBeVisible()
    expect((await pedro.snapshot()).conversations.map(c => c.Title)).not.toContain('Yesterday chat')
  })

  test('deleting another chat keeps the open one', async ({ pedro }) => {
    await pedro.open(HISTORY)
    await pedro.chat('Today chat').click()
    await pedro.chat('Ancient chat').hover()
    await pedro.chatList.getByRole('button', { name: 'Delete Ancient chat' }).click()
    await expect(pedro.chat('Ancient chat')).toHaveCount(0)
    await expect(pedro.chatTitle).toHaveText('Today chat')
    await expect(pedro.userBubbles).toHaveText(['kubernetes ingress'])
  })

  test('delete all asks for confirmation first', async ({ pedro, page }) => {
    await pedro.open(HISTORY)
    const openMenu = async () => {
      await pedro.sidebar.getByRole('button', { name: 'Chat list options' }).click()
      await page.getByRole('menuitem', { name: 'Delete all chats…' }).click()
    }

    await openMenu()
    await expect(pedro.alertDialog()).toContainText('Delete all 5 chats?')
    await pedro.alertDialog().getByRole('button', { name: 'Cancel' }).click()
    await expect(pedro.alertDialog()).toHaveCount(0)
    await expect(pedro.chatList.getByRole('listitem')).toHaveCount(5)

    await openMenu()
    await pedro.alertDialog().getByRole('button', { name: 'Delete all' }).click()
    await expect(pedro.toast('All chats deleted')).toBeVisible()
    await expect(pedro.sidebar.getByText('No chats yet')).toBeVisible()
    expect((await pedro.snapshot()).conversations).toHaveLength(0)
  })
})
