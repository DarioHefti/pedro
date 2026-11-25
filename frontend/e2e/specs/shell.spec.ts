import { expect, test } from '../fixtures'

test.describe('app shell', () => {
  test('first launch shows the welcome screen and an empty chat list', async ({ pedro, page }) => {
    await pedro.open()

    await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Pedro' })).toBeVisible()
    await expect(page.getByRole('list', { name: 'Suggestions' }).getByRole('button')).toHaveCount(4)
    await expect(pedro.sidebar.getByText('No chats yet')).toBeVisible()
    await expect(pedro.chatTitle).toHaveText('New chat')
    await expect(pedro.sendButton).toBeDisabled()
    await expect(pedro.composer).toBeFocused()
  })

  test('a suggestion fills the composer', async ({ pedro, page }) => {
    await pedro.open()
    await page.getByRole('button', { name: /Research a topic/ }).click()
    await expect(pedro.composer).toHaveValue('Research the latest developments on ')
    await expect(pedro.composer).toBeFocused()
  })

  test('a suggestion that needs a folder opens the folder picker', async ({ pedro, page }) => {
    await pedro.open({ folderSelections: ['/Users/test/projects/pedro'] })
    await page.getByRole('button', { name: /Explain a project/ }).click()
    await expect(pedro.composer).toHaveValue(/Give me a clear overview of this project/)
    await expect(page.locator('.composer-attachments').getByText('pedro', { exact: true })).toBeVisible()
    expect(await pedro.calls('SelectFolder')).toHaveLength(1)
  })

  test('the theme switch applies and survives a reload', async ({ pedro, page }) => {
    await pedro.open()
    const html = page.locator('html')

    await pedro.sidebar.getByRole('radio', { name: 'Dark theme' }).click()
    await expect(html).toHaveAttribute('data-theme', 'dark')

    await page.reload()
    await expect(pedro.composer).toBeVisible()
    await expect(html).toHaveAttribute('data-theme', 'dark')
    await expect(pedro.sidebar.getByRole('radio', { name: 'Dark theme' })).toBeChecked()

    await pedro.sidebar.getByRole('radio', { name: 'Light theme' }).click()
    await expect(html).toHaveAttribute('data-theme', 'light')
  })

  test('the sidebar hides and shows from buttons and the shortcut', async ({ pedro, page }) => {
    await pedro.open()
    const showSidebar = page.getByRole('button', { name: 'Show sidebar' })

    await pedro.sidebar.getByRole('button', { name: 'Hide sidebar' }).click()
    await expect(showSidebar).toBeVisible()

    await showSidebar.click()
    await expect(showSidebar).toHaveCount(0)
    await expect(pedro.sidebar.getByRole('button', { name: 'New chat' })).toBeVisible()

    await pedro.shortcut('b')
    await expect(showSidebar).toBeVisible()
    await pedro.shortcut('b')
    await expect(showSidebar).toHaveCount(0)
  })

  test('the new-chat shortcut leaves the open chat', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.ask('Hello Pedro')
    await expect(pedro.chatTitle).toHaveText('Hello Pedro')

    await pedro.shortcut('n')
    await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Pedro' })).toBeVisible()
    await expect(pedro.chatTitle).toHaveText('New chat')
    await expect(pedro.composer).toBeFocused()
  })
})
