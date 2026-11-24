/**
 * Installs the fake Go backend as `window.go` / `window.runtime`, exactly where the Wails
 * webview puts the real bridge. Injected by vite.config.ts in `mock` and `e2e` modes only.
 *
 * A test seeds it by setting `window.__PEDRO_FAKE_SEED__` before the page loads.
 */
import { createFakeBackend } from './backend'
import { demoSeed } from './seeds'

const seed = window.__PEDRO_FAKE_SEED__ ?? (import.meta.env.MODE === 'mock' ? demoSeed : {})
const backend = createFakeBackend(seed)

window.go = { main: { App: backend.app, Updater: backend.updater } }
window.runtime = backend.runtime
window.__pedroFake = backend.control
