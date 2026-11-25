import { test as base, expect, type Locator, type Page } from '@playwright/test'
import type { FakeSeed, FakeSnapshot, RecordedCall, ReplyScript } from './fake-backend/types'

export { expect }
export type { FakeSeed, ReplyScript }

/** Page object for the Pedro window, plus handles on the fake backend behind it. */
export class PedroApp {
  constructor(readonly page: Page) {}

  /** Seeds the fake backend, loads the app and waits until it is interactive. */
  async open(seed: FakeSeed = {}): Promise<void> {
    await this.page.addInitScript(s => {
      window.__PEDRO_FAKE_SEED__ = s
    }, seed)
    await this.page.goto('/')
    await expect(this.composer).toBeVisible()
  }

  // ------------------------------------------------------------------ regions

  get sidebar(): Locator {
    return this.page.getByRole('complementary', { name: 'Chats' })
  }

  get chatList(): Locator {
    return this.page.getByRole('navigation', { name: 'Chat history' })
  }

  get chatTitle(): Locator {
    return this.page.locator('.chat-title')
  }

  get composer(): Locator {
    return this.page.getByRole('textbox', { name: 'Message Pedro' })
  }

  get sendButton(): Locator {
    return this.page.getByRole('button', { name: 'Send message' })
  }

  get stopButton(): Locator {
    return this.page.getByRole('button', { name: 'Stop generating' })
  }

  get userMessages(): Locator {
    return this.page.getByRole('article', { name: 'You', exact: true })
  }

  get replies(): Locator {
    return this.page.getByRole('article', { name: 'Pedro', exact: true })
  }

  /** The text bubbles of the person's messages (without their action buttons). */
  get userBubbles(): Locator {
    return this.userMessages.locator('.bubble')
  }

  /** The text bubbles of Pedro's replies; tool-only steps have none. */
  get replyBubbles(): Locator {
    return this.replies.locator('.bubble')
  }

  get streamingReply(): Locator {
    return this.page.getByRole('article', { name: 'Pedro is replying' })
  }

  /** The sidebar row for a chat. */
  chat(title: string): Locator {
    return this.chatList.getByRole('button', { name: title, exact: true })
  }

  /** A visible toast notification containing `text`. */
  toast(text: string | RegExp): Locator {
    return this.page.locator('[data-slot="toast-region"]').getByRole('listitem').filter({ hasText: text })
  }

  dialog(name: string | RegExp): Locator {
    return this.page.getByRole('dialog', { name })
  }

  alertDialog(): Locator {
    return this.page.getByRole('alertdialog')
  }

  // ------------------------------------------------------------------ actions

  async send(text: string): Promise<void> {
    await this.composer.fill(text)
    await this.composer.press('Enter')
  }

  /** Sends a message and waits for the reply to finish streaming. */
  async ask(text: string): Promise<void> {
    const before = await this.replies.count()
    await this.send(text)
    await this.waitForIdle()
    await expect(this.replies).not.toHaveCount(before)
  }

  async waitForIdle(): Promise<void> {
    await expect(this.streamingReply).toHaveCount(0)
    await expect(this.stopButton).toHaveCount(0)
  }

  /** Presses a shortcut with the platform's primary modifier (⌘ on macOS, Ctrl elsewhere). */
  async shortcut(keys: string): Promise<void> {
    await this.page.keyboard.press(`ControlOrMeta+${keys}`)
  }

  // ------------------------------------------------------------- fake backend

  queueReply(reply: ReplyScript): Promise<void> {
    return this.page.evaluate(r => window.__pedroFake!.queueReply(r), reply)
  }

  queueFileSelection(path: string): Promise<void> {
    return this.page.evaluate(p => window.__pedroFake!.queueFileSelection(p), path)
  }

  queueFolderSelection(path: string): Promise<void> {
    return this.page.evaluate(p => window.__pedroFake!.queueFolderSelection(p), path)
  }

  dropFiles(paths: string[]): Promise<void> {
    return this.page.evaluate(p => window.__pedroFake!.dropFiles(p), paths)
  }

  emit(event: string, ...data: unknown[]): Promise<void> {
    return this.page.evaluate(([e, d]) => window.__pedroFake!.emit(e as string, ...(d as unknown[])), [event, data] as const)
  }

  calls(method?: string): Promise<RecordedCall[]> {
    return this.page.evaluate(m => window.__pedroFake!.calls(m), method)
  }

  snapshot(): Promise<FakeSnapshot> {
    return this.page.evaluate(() => window.__pedroFake!.snapshot())
  }
}

export const test = base.extend<{ pedro: PedroApp; failOnPageErrors: void }>({
  pedro: async ({ page }, use) => {
    await use(new PedroApp(page))
  },
  /** Any uncaught exception in the page fails the test, even when the UI looks right. */
  failOnPageErrors: [
    async ({ page }, use) => {
      const errors: string[] = []
      page.on('pageerror', err => errors.push(err.stack ?? err.message))
      await use()
      expect(errors, 'uncaught errors in the page').toEqual([])
    },
    { auto: true },
  ],
})
