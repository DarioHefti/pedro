import type { FakeControl, FakeSeed } from './types'

declare global {
  interface Window {
    /** Seed for the fake backend; set it before the page loads. */
    __PEDRO_FAKE_SEED__?: FakeSeed
    /** Control surface of the fake backend, for tests. */
    __pedroFake?: FakeControl
    go?: { main: { App: unknown; Updater: unknown } }
    runtime?: unknown
  }
}

export {}
