import { defineConfig, devices } from '@playwright/test'

/**
 * End-to-end tests for the Pedro UI. The app runs in a regular browser against the in-memory
 * fake Go backend (e2e/fake-backend), so no Wails window, database or LLM is involved.
 *
 * By default the suite runs against a production build (`vite build --mode e2e`), which is
 * what ships. Set E2E_DEV=1 to run against the Vite dev server instead (faster to iterate).
 */
const PORT = Number(process.env.E2E_PORT ?? 4179)
const CI = !!process.env.CI

const server = process.env.E2E_DEV
  ? `npx vite --mode e2e --port ${PORT} --strictPort`
  : `npx vite build --mode e2e --outDir e2e/.dist --emptyOutDir --logLevel warn && npx vite preview --mode e2e --outDir e2e/.dist --port ${PORT} --strictPort`

export default defineConfig({
  testDir: './e2e/specs',
  fullyParallel: true,
  forbidOnly: CI,
  retries: CI ? 1 : 0,
  workers: CI ? 2 : undefined,
  reporter: CI ? [['github'], ['html', { open: 'never' }]] : [['list'], ['html', { open: 'never' }]],
  expect: { timeout: 7_500 },
  use: {
    baseURL: `http://localhost:${PORT}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1180, height: 780 },
        permissions: ['clipboard-read', 'clipboard-write'],
      },
    },
  ],
  webServer: {
    command: server,
    url: `http://localhost:${PORT}`,
    reuseExistingServer: !CI,
    timeout: 180_000,
  },
})
