import { expect, test, type PedroApp } from '../fixtures'

/** 1×1 transparent PNG. */
const PNG_BASE64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII='

function attachments(pedro: PedroApp) {
  return pedro.page.locator('.composer-attachments')
}

async function openAttachMenu(pedro: PedroApp) {
  await pedro.page.getByRole('button', { name: 'Attach a file or folder — or paste and drop' }).click()
}

test.describe('attachments', () => {
  test('a file attached by path is sent as a reference the model can read', async ({ pedro, page }) => {
    await pedro.open({ fileSelections: ['/Users/test/notes.md'] })

    await openAttachMenu(pedro)
    await page.getByRole('menuitem', { name: /File…/ }).click()
    await expect(attachments(pedro).getByText('notes.md')).toBeVisible()
    await expect(attachments(pedro).getByText('File path')).toBeVisible()

    await pedro.ask('Summarize this file')

    const sent = pedro.userMessages.first()
    await expect(sent.getByRole('button', { name: 'Open file notes.md' })).toBeVisible()
    await expect(sent).toContainText('Summarize this file')
    await expect(sent).not.toContainText('[Path:')
    await expect(attachments(pedro)).toHaveCount(0)

    const [send] = await pedro.calls('SendMessage')
    expect(send.args[1]).toContain('[File: notes.md]\n[Path: /Users/test/notes.md]')
    expect(JSON.parse(String(send.args[2]))).toEqual([{ type: 'file-ref', content: '/Users/test/notes.md', name: 'notes.md' }])

    await expect(page.getByRole('button', { name: /Used 1 tool/ })).toContainText('Read file')
    await expect(pedro.chat('Summarize this file')).toBeVisible()

    await sent.getByRole('button', { name: 'Open file notes.md' }).click()
    await expect.poll(async () => (await pedro.calls('OpenPath')).map(c => c.args[0])).toEqual(['/Users/test/notes.md'])
  })

  test('a folder attached by path asks the model to browse it', async ({ pedro, page }) => {
    await pedro.open({ folderSelections: ['/Users/test/projects/site'] })
    await openAttachMenu(pedro)
    await page.getByRole('menuitem', { name: /Folder…/ }).click()
    await expect(attachments(pedro).getByText('Folder', { exact: true })).toBeVisible()

    await pedro.ask('What is in here?')

    const [send] = await pedro.calls('SendMessage')
    expect(send.args[1]).toContain('[Folder: site]\n[Path: /Users/test/projects/site]')
    await expect(pedro.userMessages.first().getByRole('button', { name: 'Open folder site' })).toBeVisible()
    await expect(page.getByRole('button', { name: /Used 1 tool/ })).toContainText('Browse folder')
  })

  test('cancelling the picker attaches nothing', async ({ pedro, page }) => {
    await pedro.open()
    await openAttachMenu(pedro)
    await page.getByRole('menuitem', { name: /File…/ }).click()
    await expect.poll(async () => (await pedro.calls('SelectFile')).length).toBe(1)
    await expect(attachments(pedro)).toHaveCount(0)
  })

  test('an attachment can be removed before sending', async ({ pedro, page }) => {
    await pedro.open({ fileSelections: ['/Users/test/a.txt'] })
    await openAttachMenu(pedro)
    await page.getByRole('menuitem', { name: /File…/ }).click()
    await page.getByRole('button', { name: 'Remove a.txt' }).click()
    await expect(attachments(pedro)).toHaveCount(0)
  })

  test('files dropped onto the window are attached by path', async ({ pedro }) => {
    await pedro.open()
    await pedro.dropFiles(['/Users/test/report.pdf', '/Users/test/voice.mp3'])
    await expect(attachments(pedro).getByText('report.pdf')).toBeVisible()
    await expect(attachments(pedro).getByText('voice.mp3')).toBeVisible()

    await pedro.ask('Summarize both')
    const [send] = await pedro.calls('SendMessage')
    expect(send.args[1]).toContain('use parse_document')
    expect(send.args[1]).toContain('Use the transcribe_audio tool with this path')
  })

  test('a pasted image is sent to the model for vision', async ({ pedro, page }) => {
    await pedro.open()
    await page.evaluate(async base64 => {
      const bytes = Uint8Array.from(atob(base64), c => c.charCodeAt(0))
      const data = new DataTransfer()
      data.items.add(new File([bytes], 'screenshot.png', { type: 'image/png' }))
      document.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data }))
    }, PNG_BASE64)

    await expect(attachments(pedro).locator('img.attachment-thumb')).toBeVisible()
    await pedro.ask('What is in this image?')

    const [send] = await pedro.calls('SendMessageWithImages')
    expect(send.args[2]).toEqual([`data:image/png;base64,${PNG_BASE64}`])
    expect(await pedro.calls('SendMessage')).toHaveLength(0)
    await expect(pedro.userMessages.first().getByRole('button', { name: 'View attached image 1' })).toBeVisible()
  })
})
