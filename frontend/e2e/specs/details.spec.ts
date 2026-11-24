import { expect, test, type PedroApp } from '../fixtures'

function details(pedro: PedroApp) {
  return pedro.dialog('Request details')
}

test.describe('request details', () => {
  test('shows the payload of the last request for a chat', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.ask('Please research the latest Wails release')

    await page.getByRole('button', { name: '2 requests' }).click()
    const dlg = details(pedro)
    await expect(dlg).toBeVisible()

    const entry = dlg.getByRole('option', { name: /Please research the latest Wails release/ })
    await expect(entry).toHaveAttribute('aria-selected', 'true')
    await expect(entry).toContainText('pedro-test-model')
    await expect(dlg.getByRole('heading', { level: 3, name: 'Please research the latest Wails release' })).toBeVisible()

    const conversation = dlg.getByRole('tabpanel', { name: /Conversation/ })
    await expect(conversation.getByRole('article').first()).toContainText('You are Pedro')
    await expect(conversation).toContainText('Please research the latest Wails release')

    await dlg.getByRole('tab', { name: /Tools/ }).click()
    const tools = dlg.getByRole('tabpanel', { name: /Tools/ })
    await expect(tools).toContainText('web_search')
    await expect(tools).toContainText('read_file')

    await dlg.getByRole('tab', { name: 'Raw JSON' }).click()
    await expect(dlg.getByRole('tabpanel', { name: 'Raw JSON' })).toContainText('"model": "pedro-test-model"')
  })

  test('the shortcut opens it, search filters it and clear all empties it', async ({ pedro, page }) => {
    await pedro.open()
    await pedro.ask('First chat question')
    await pedro.shortcut('n')
    await pedro.ask('Second chat question')

    await pedro.shortcut('Shift+D')
    const dlg = details(pedro)
    await expect(dlg.getByRole('option')).toHaveCount(2)

    const search = dlg.getByRole('textbox', { name: /Search requests/ })
    await search.fill('first chat')
    await expect(dlg.getByRole('option')).toHaveCount(1)
    await search.fill('no such request')
    await expect(dlg.getByText('No matches for “no such request”')).toBeVisible()
    await search.fill('')

    await dlg.getByRole('button', { name: 'Clear all' }).click()
    await expect(pedro.alertDialog()).toContainText('Clear all 2 captured requests?')
    await pedro.alertDialog().getByRole('button', { name: 'Clear all' }).click()
    await expect(pedro.toast('Request details cleared')).toBeVisible()
    await expect(dlg.getByText('No requests recorded yet')).toBeVisible()
    expect((await pedro.snapshot()).llmDetails).toBe(0)
    await page.keyboard.press('Escape')
    await expect(dlg).toHaveCount(0)
  })
})
