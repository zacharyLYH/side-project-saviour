import { defineConfig } from '@playwright/test'

// Two layers live here:
//
// 1. Behavioral tests (app.spec.ts, terminal.visual.spec.ts): API/WS mocked
//    at the network layer (page.route / page.routeWebSocket), no backend.
//    They need no Go toolchain and no Docker.
//
// 2. The full-stack test (terminal.stack.spec.ts) boots the real Go server
//    as a second webServer — login included, nothing mocked; the PIN is read
//    from the console-mailer output redirected into test-results/sps-stack-
//    server.log. It needs the Go toolchain and a running Docker engine (it
//    skips itself when the engine is down), and it owns port 8080 while it
//    runs.
const dataDir = 'test-results/sps-stack-data'
const serverLog = 'test-results/sps-stack-server.log'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  use: {
    // Locally use the installed Chrome (no browser download); in CI use
    // Playwright's bundled Chromium (npx playwright install chromium).
    channel: process.env.CI ? 'chromium' : 'chrome',
    baseURL: 'http://localhost:5173',
  },
  webServer: [
    {
      command:
        `mkdir -p ${dataDir} && SPS_BIND=127.0.0.1:8080 ` +
        `SPS_LOGIN_EMAIL=me@example.com SPS_DATA_DIR=$PWD/${dataDir} ` +
        // empty-but-present shadows the repo-root .env, forcing the
        // console mailer so the test can read the PIN from its log
        `SMTP_USER= SMTP_PASSWORD= ` +
        `go -C ../server run ./cmd/server 2> ${serverLog}`,
      url: 'http://localhost:8080/health',
      reuseExistingServer: false,
      timeout: 60_000,
      stdout: 'ignore',
      stderr: 'pipe',
    },
    {
      command: 'npm run dev',
      url: 'http://localhost:5173',
      reuseExistingServer: true,
      timeout: 60_000,
    },
  ],
})
