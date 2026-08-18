import { defineConfig } from '@playwright/test'

// Behavioral tests only (no visual/screenshot tests yet). API calls are
// mocked at the network layer with page.route, so no real backend is booted:
// these tests need no Go toolchain and no .env. The real server + Vite proxy
// wiring is covered by the docker-compose CI job.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  use: {
    // Locally use the installed Chrome (no browser download); in CI use
    // Playwright's bundled Chromium (npx playwright install chromium).
    channel: process.env.CI ? 'chromium' : 'chrome',
    baseURL: 'http://localhost:5173',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 60_000,
  },
})
