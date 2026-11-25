import { expect, test, type FakeSeed, type PedroApp } from '../fixtures'

function settings(pedro: PedroApp) {
  return pedro.dialog('Settings')
}

async function openSettings(pedro: PedroApp, section?: string) {
  await pedro.sidebar.getByRole('button', { name: 'Settings' }).click()
  await expect(settings(pedro)).toBeVisible()
  if (section) await settings(pedro).getByRole('tab', { name: section }).click()
}

/** Closes a saved settings dialog with its footer button (Escape first dismisses a visible toast). */
async function closeSettings(pedro: PedroApp) {
  await settings(pedro).locator('.dialog-footer').getByRole('button', { name: 'Close' }).click()
  await expect(settings(pedro)).toHaveCount(0)
}

async function save(pedro: PedroApp) {
  await settings(pedro).getByRole('button', { name: 'Save changes' }).click()
  await expect(pedro.toast('Settings saved')).toBeVisible()
  await expect(settings(pedro).getByText('All changes saved')).toBeVisible()
}

test.describe('settings', () => {
  test('opens from the sidebar and the shortcut, and shows the version', async ({ pedro, page }) => {
    await pedro.open({ version: '1.6.0' })
    await openSettings(pedro)
    await expect(settings(pedro).getByRole('tab', { name: 'Connection' })).toHaveAttribute('aria-selected', 'true')
    await expect(settings(pedro)).toContainText('Pedro v1.6.0')
    await page.keyboard.press('Escape')
    await expect(settings(pedro)).toHaveCount(0)

    await pedro.shortcut(',')
    await expect(settings(pedro)).toBeVisible()
  })

  test('switching provider shows its fields and saves them', async ({ pedro }) => {
    await pedro.open({ settings: { authenticated: 'true' } })
    await openSettings(pedro)
    const dlg = settings(pedro)
    await expect(dlg.getByRole('textbox', { name: 'Base URL' })).toHaveValue('http://localhost:1234/v1')

    await dlg.getByRole('combobox', { name: 'Provider' }).click()
    await pedro.page.getByRole('option', { name: /^OpenAI/ }).click()
    await expect(dlg.getByRole('textbox', { name: 'Base URL' })).toHaveCount(0)
    await expect(dlg.getByRole('textbox', { name: 'Model' })).toHaveValue('gpt-4o')
    await dlg.getByRole('textbox', { name: 'API key' }).fill('sk-test-123')
    await expect(dlg.getByText('Unsaved changes').first()).toBeVisible()
    await expect(dlg.getByRole('button', { name: 'Save & test' })).toBeVisible()

    await save(pedro)
    const { settings: stored } = await pedro.snapshot()
    expect(stored).toMatchObject({ provider_type: 'openai', openai_api_key: 'sk-test-123', openai_model: 'gpt-4o', authenticated: 'false' })
  })

  test('the save shortcut saves', async ({ pedro, page }) => {
    await pedro.open()
    await openSettings(pedro)
    await settings(pedro).getByRole('textbox', { name: 'Model' }).fill('qwen-3-coder')
    await page.keyboard.press('ControlOrMeta+s')
    await expect(pedro.toast('Settings saved')).toBeVisible()
    expect((await pedro.snapshot()).settings.compat_model).toBe('qwen-3-coder')
  })

  test('closing with unsaved changes asks before discarding them', async ({ pedro, page }) => {
    await pedro.open()
    await openSettings(pedro)
    await settings(pedro).getByRole('textbox', { name: 'Model' }).fill('something-else')

    await page.keyboard.press('Escape')
    await expect(pedro.alertDialog()).toContainText('Discard unsaved changes?')
    await pedro.alertDialog().getByRole('button', { name: 'Keep editing' }).click()
    await expect(settings(pedro).getByRole('textbox', { name: 'Model' })).toHaveValue('something-else')

    await settings(pedro).getByRole('button', { name: 'Cancel' }).click()
    await pedro.alertDialog().getByRole('button', { name: 'Discard changes' }).click()
    await expect(settings(pedro)).toHaveCount(0)
    expect((await pedro.snapshot()).settings.compat_model).toBe('pedro-test-model')

    await openSettings(pedro)
    await expect(settings(pedro).getByRole('textbox', { name: 'Model' })).toHaveValue('pedro-test-model')
  })

  test('a passing connection test is remembered for these settings', async ({ pedro }) => {
    await pedro.open()
    await openSettings(pedro)
    await expect(settings(pedro).getByText('Not tested yet')).toBeVisible()

    await settings(pedro).getByRole('button', { name: 'Test connection' }).click()
    await expect(pedro.toast('Connection successful!')).toBeVisible()
    await expect(settings(pedro).getByText('Connected', { exact: true })).toBeVisible()

    await closeSettings(pedro)
    await openSettings(pedro)
    await expect(settings(pedro).getByText('Connected', { exact: true })).toBeVisible()

    // Changing a connection setting invalidates the stored result.
    await settings(pedro).getByRole('textbox', { name: 'Model' }).fill('other-model')
    await save(pedro)
    await expect(settings(pedro).getByText('Not tested yet')).toBeVisible()
  })

  test('a failing connection test shows why', async ({ pedro }) => {
    await pedro.open({ connectionTest: { error: '401 Unauthorized: invalid API key' } })
    await openSettings(pedro)
    await settings(pedro).getByRole('button', { name: 'Test connection' }).click()
    await expect(pedro.toast('Connection failed')).toContainText('401 Unauthorized: invalid API key')
    await expect(settings(pedro).locator('.connection-card')).toContainText('401 Unauthorized: invalid API key')
  })

  test('azure sign in and sign out', async ({ pedro }) => {
    const seed: FakeSeed = {
      settings: { provider_type: 'azure', azure_endpoint: 'https://contoso.openai.azure.com', azure_deployment: 'gpt-4o' },
    }
    await pedro.open(seed)
    await openSettings(pedro)
    const dlg = settings(pedro)
    await expect(dlg.getByText('Not signed in')).toBeVisible()

    await dlg.getByRole('button', { name: 'Sign in with Azure' }).click()
    await expect(pedro.toast('Signed in to Azure')).toBeVisible()
    await expect(dlg.getByText('Signed in', { exact: true })).toBeVisible()

    await dlg.getByRole('button', { name: 'Sign out' }).click()
    await expect(pedro.toast('Signed out')).toBeVisible()
    await expect(dlg.getByText('Not signed in')).toBeVisible()
    expect(await pedro.calls('SignOut')).toHaveLength(1)
  })

  test('instructions save, and the system prompt can be reset', async ({ pedro }) => {
    await pedro.open()
    await openSettings(pedro, 'Instructions')
    const dlg = settings(pedro)

    await dlg.getByRole('textbox', { name: 'Additional instructions' }).fill('Always answer in haiku.')
    await dlg.getByRole('button', { name: /System prompt/ }).click()
    const prompt = dlg.locator('textarea.prompt-editor')
    await expect(prompt).toHaveValue(/You are Pedro/)
    await prompt.fill('You are a pirate.')
    await expect(dlg.getByRole('button', { name: /System prompt/ })).toContainText('Customized')
    await save(pedro)
    expect((await pedro.snapshot()).settings).toMatchObject({ custom_system_prompt: 'Always answer in haiku.', system_prompt: 'You are a pirate.' })

    await dlg.getByRole('button', { name: 'Reset to default' }).click()
    await expect(prompt).toHaveValue(/You are Pedro/)
    await expect(dlg.getByRole('button', { name: /System prompt/ })).toContainText('Default')
  })

  test('the welcome message changes the empty state', async ({ pedro, page }) => {
    await pedro.open()
    await openSettings(pedro, 'Appearance')
    await settings(pedro).getByRole('textbox', { name: 'Welcome message' }).fill('Hola, soy Pedro')
    await save(pedro)
    await closeSettings(pedro)
    await expect(page.getByRole('heading', { level: 1, name: 'Hola, soy Pedro' })).toBeVisible()
  })

  test('appearance previews live and saves accent and text size', async ({ pedro, page }) => {
    await pedro.open()
    await openSettings(pedro, 'Appearance')
    const dlg = settings(pedro)
    const rootStyle = (prop: string) => page.evaluate(p => document.documentElement.style.getPropertyValue(p), prop)

    await dlg.getByRole('slider', { name: 'Interface' }).focus()
    await page.keyboard.press('ArrowRight')
    await expect(dlg.getByRole('slider', { name: 'Interface' })).toHaveValue('14')
    await dlg.getByRole('button', { name: 'Ocean' }).click()
    await expect(dlg.getByRole('button', { name: 'Ocean' })).toHaveAttribute('aria-pressed', 'true')
    await expect.poll(() => rootStyle('--custom-ui-font-size')).toBe('14px')

    await save(pedro)
    const stored = (await pedro.snapshot()).settings
    expect(stored.design_ui_font_size_px).toBe('14')
    expect(stored.design_light_base_color).toMatch(/^#[0-9a-f]{6}$/i)
    expect(stored.design_light_base_color).not.toBe('#242f62')

    await dlg.getByRole('button', { name: 'Reset appearance to defaults' }).click()
    await expect(dlg.getByRole('button', { name: 'Pedro' })).toHaveAttribute('aria-pressed', 'true')
    await expect(dlg.getByRole('slider', { name: 'Interface' })).toHaveValue('13')
  })

  test('memories can be added, edited and forgotten', async ({ pedro }) => {
    await pedro.open({ memories: [{ key: 'editor', value: 'VS Code', category: 'work', importance: 5 }] })
    await openSettings(pedro, 'Memory')
    const dlg = settings(pedro)
    const items = dlg.locator('.settings-list > li')
    await expect(items).toHaveCount(1)
    await expect(items.first()).toContainText('Critical')

    await dlg.getByRole('textbox', { name: 'Key', exact: true }).fill('home_city')
    await dlg.getByRole('textbox', { name: 'Value', exact: true }).fill('Zurich')
    await dlg.getByRole('textbox', { name: 'Category Optional' }).fill('personal')
    await dlg.getByRole('button', { name: 'Add' }).click()
    await expect(items).toHaveCount(2)
    await expect(items.filter({ hasText: 'home_city' })).toContainText('Zurich')
    await expect(dlg.getByRole('textbox', { name: 'Key', exact: true })).toHaveValue('')

    const row = items.filter({ hasText: 'home_city' })
    await row.getByRole('button', { name: 'Edit' }).click()
    const editing = dlg.locator('.settings-list > li[data-editing]')
    await editing.getByRole('textbox', { name: 'Value', exact: true }).fill('Bern')
    await editing.getByRole('button', { name: 'Save' }).click()
    await expect(items.filter({ hasText: 'home_city' })).toContainText('Bern')

    await items.filter({ hasText: 'editor' }).getByRole('button', { name: 'Forget' }).click()
    await expect(pedro.toast('Forgot “editor”')).toBeVisible()
    await expect(items).toHaveCount(1)
    expect((await pedro.snapshot()).memories.map(m => [m.Key, m.Value])).toEqual([['home_city', 'Bern']])
  })

  test('usage shows lifetime totals and links to request details', async ({ pedro }) => {
    await pedro.open({ stats: { globalRequests: 1234, lifetimeTokens: 98_765 } })
    await openSettings(pedro, 'Usage')
    const dlg = settings(pedro)
    await expect(dlg.locator('.stat-card').first()).toContainText('1,234')
    await expect(dlg.locator('.stat-card').nth(1)).toContainText('98,765')
    await expect(dlg.locator('.stat-card').nth(1)).toContainText('≈ 80 per request')

    await dlg.getByRole('button', { name: 'Request details' }).click()
    await expect(pedro.dialog('Request details')).toBeVisible()
  })
})
