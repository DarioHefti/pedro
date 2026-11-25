/// <reference types="vitest/config" />
import { execSync } from 'node:child_process'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import { GENERATED_IMAGES_ROUTE, PASTED_IMAGES_ROUTE } from './src/utils/fileKinds'

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

/**
 * The git tag of the source tree: "v1.8.0" on the tag, "v1.8.0-3-gf60f48d" three commits later.
 * Release builds stamp their version into the Go binary; `wails dev` builds cannot (Wails
 * builds with -buildvcs=false) and report "dev", so the settings show this instead. Empty
 * without git or tags, e.g. in a shallow CI checkout.
 */
function sourceVersion(): string {
  try {
    return execSync('git describe --tags', { stdio: ['ignore', 'pipe', 'ignore'] }).toString().trim()
  } catch {
    return ''
  }
}

/** URL prefixes the Go backend serves itself (App.assetHandler), not this dev server. */
const BACKEND_ROUTES = [GENERATED_IMAGES_ROUTE, PASTED_IMAGES_ROUTE]

/**
 * Under `wails dev`, Wails asks this dev server for every GET and only falls back to the Go
 * asset handler when the answer is 404. Vite's SPA fallback answers any unknown path with
 * index.html instead, so generated and pasted images would load as HTML and never show.
 * Answering 404 for the backend's routes hands them to Go.
 */
function backendRoutes(): Plugin {
  return {
    name: 'pedro-backend-routes',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.url && BACKEND_ROUTES.some(route => req.url?.startsWith(route))) {
          res.statusCode = 404
          res.end()
          return
        }
        next()
      })
    },
  }
}

export default defineConfig(({ mode }) => ({
  plugins: [react(), backendRoutes(), ...(FAKE_BACKEND_MODES.has(mode) ? [fakeBackend()] : [])],
  define: {
    __PEDRO_SOURCE_VERSION__: JSON.stringify(sourceVersion()),
  },
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
