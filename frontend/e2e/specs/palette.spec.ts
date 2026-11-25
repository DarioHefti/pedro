import { expect, test, type FakeSeed, type PedroApp } from '../fixtures'

const SEED: FakeSeed = {
  conversations: [
    { title: 'Holiday planning', daysAgo: 0, messages: [{ role: 'user', content: 'Plan a trip' }, { role: 'assistant', content: 'Visit the glacier express route.' }] },
    { title: 'Budget review', daysAgo: 2, messages: [{ role: 'user', content: 'Review the budget' }, { role: 'assistant', content: 'Spending is on track.' }] },
  ],
}

function palette(pedro: PedroApp) {
  return pedro.dialog('Command palette')
}

async function openPalette(pedro: PedroApp) {
  await pedro.shortcut('k')
  await expect(palette(pedro)).toBeVisible()
  await expect(palette(pedro).getByRole('combobox', { name: 'Search chats and actions' })).toBeFocused()
}

async function run(pedro: PedroApp, query: string, option: string | RegExp) {
  await openPalette(pedro)
  await pedro.page.keyboard.type(query)
  await expect(palette(pedro).getByRole('option', { name: option })).toBeVisible()
  await palette(pedro).getByRole('option', { name: option }).click()
  await expect(palette(pedro)).toHaveCount(0)
}

test.describe('command palette', () => {
  test('opens from the header button and closes with escape', async ({ pedro, page }) => {
    await pedro.open(SEED)
    await page.getByRole('button', { name: 'Command palette' }).click()
    await expect(palette(pedro).getByRole('option', { name: 'New chat' })).toBeVisible()
    await expect(palette(pedro).getByRole('group', { name: 'Chats' }).getByRole('option')).toHaveCount(2)
    await page.keyboard.press('Escape')
    await expect(palette(pedro)).toHaveCount(0)
  })

  test('runs actions', async ({ pedro, page }) => {
    await pedro.open(SEED)

    await run(pedro, 'settings', 'Open settings')
    await expect(pedro.dialog('Settings')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(pedro.dialog('Settings')).toHaveCount(0)

    await run(pedro, 'dark', 'Switch to dark theme')
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')

    await run(pedro, 'sidebar', 'Toggle sidebar')
    await expect(page.getByRole('button', { name: 'Show sidebar' })).toBeVisible()

    await run(pedro, 'payload', 'Request details')
    await expect(pedro.dialog('Request details')).toBeVisible()
  })

  test('jumps to a chat by title', async ({ pedro }) => {
    await pedro.open(SEED)
    await openPalette(pedro)
    await pedro.page.keyboard.type('budget')
    await pedro.page.keyboard.press('Enter')
    await expect(pedro.chatTitle).toHaveText('Budget review')
    await expect(pedro.replyBubbles).toHaveText([/Spending is on track/])
  })

  test('finds chats by message content', async ({ pedro }) => {
    await pedro.open(SEED)
    await openPalette(pedro)
    await pedro.page.keyboard.type('glacier')
    const hits = palette(pedro).getByRole('group', { name: 'Found in messages' })
    await expect(hits.getByRole('option')).toHaveCount(1)
    await expect(hits).toContainText('Visit the glacier express route.')
    await hits.getByRole('option').click()
    await expect(pedro.chatTitle).toHaveText('Holiday planning')
  })

  test('starts a new chat', async ({ pedro, page }) => {
    await pedro.open(SEED)
    await pedro.chat('Budget review').click()
    await run(pedro, 'new', 'New chat')
    await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Pedro' })).toBeVisible()
  })
})
