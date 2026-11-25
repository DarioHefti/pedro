import { expect, test } from '../fixtures'

// UpdateNotification skips the update check in dev builds.
test.skip(!!process.env.E2E_DEV, 'update checks only run in production builds')

test.describe('updates', () => {
  test('an available update can be installed', async ({ pedro, page }) => {
    await pedro.open({ update: { latestVersion: '9.9.9' } })
    const card = page.getByRole('status').filter({ hasText: 'Pedro v9.9.9 is available' })
    await expect(card).toBeVisible({ timeout: 10_000 })

    await card.getByRole('button', { name: 'Update now' }).click()
    await expect(page.getByText('Update installed')).toBeVisible()
    await expect(page.getByText('Restart Pedro to use the new version.')).toBeVisible()
    const [install] = await pedro.calls('DownloadAndInstall')
    expect(install.args).toEqual(['https://example.com/Pedro-macos-arm64.dmg', 'Pedro-macos-arm64.dmg'])
  })

  test('an update can be postponed', async ({ pedro, page }) => {
    await pedro.open({ update: { latestVersion: '9.9.9' } })
    const card = page.getByRole('status').filter({ hasText: 'is available' })
    await expect(card).toBeVisible({ timeout: 10_000 })
    await card.getByRole('button', { name: 'Later' }).click()
    await expect(card).toHaveCount(0)
    expect(await pedro.calls('DownloadAndInstall')).toHaveLength(0)
  })

  test('no card when Pedro is up to date', async ({ pedro, page }) => {
    await pedro.open()
    await expect.poll(async () => (await pedro.calls('CheckForUpdate')).length, { timeout: 10_000 }).toBe(1)
    await expect(page.getByText(/is available/)).toHaveCount(0)
  })
})
