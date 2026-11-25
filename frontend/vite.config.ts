/// <reference types="vitest/config" />
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

/**
 * Modes that run the UI against the in-browser fake Go backend (e2e/fake-backend) instead of
 * the Wails bridge: `mock` for `npm run dev:mock`, `e2e` for the Playwright suite.
 */
const FAKE_BACKEND_MODES = new Set(['mock', 'e2e'])

/**
 * Injects the fake backend ahead of the app entry. `order: 'pre'` lets Vite bundle the injected
 * module, and an earlier module script always evaluates first, so `window.go` and
 * `window.runtime` exist before any app code runs. Production builds never include it.
 */
function fakeBackend(): Plugin {
  return {
    name: 'pedro-fake-backend',
    transformIndexHtml: {
      order: 'pre',
      handler: () => [
        { tag: 'script', attrs: { type: 'module', src: '/e2e/fake-backend/install.ts' }, injectTo: 'head' },
      ],
    },
  }
}

export default defineConfig(({ mode }) => ({
  plugins: [react(), ...(FAKE_BACKEND_MODES.has(mode) ? [fakeBackend()] : [])],
  server: {
    // Test output lands inside the project; changes there must not reload a running `wails dev`.
    watch: { ignored: ['**/playwright-report/**', '**/test-results/**', '**/blob-report/**', '**/e2e/.dist/**'] },
  },
  build: {
    // Vite 4's default browser target: keeps older system webviews (WKWebView, webkit2gtk) supported.
    target: ['es2020', 'edge88', 'firefox78', 'chrome87', 'safari14'],
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        // Markdown rendering is needed for the first message. Mermaid stays out of this chunk:
        // mermaidRenderer loads it on demand with the first diagram.
        manualChunks: {
          streamdown: ['streamdown', '@streamdown/code'],
        },
      },
    },
  },
  test: {
    include: ['src/**/*.test.ts', 'e2e/fake-backend/**/*.test.ts'],
    environment: 'node',
  },
}))
