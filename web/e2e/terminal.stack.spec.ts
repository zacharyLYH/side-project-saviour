import { readFileSync } from 'node:fs'
import { expect, test, type Page } from '@playwright/test'

// Full-stack user journey: real Go server (booted by playwright.config as a
// webServer) + Vite dev proxy + real login. NOTHING is mocked — even the PIN
// comes from the console-mailer output redirected to
// test-results/sps-stack-server.log. Skips itself when Docker is unavailable.
//
// The rule that keeps this test honest: no API shortcuts for state the UI
// can create itself. It once pre-created the tmux session via a raw POST and
// thereby masked that clicking Terminal was broken for real users — the UI
// path IS the test path now.

test('login, create a project in the UI, open its terminal, type', async ({ page }) => {
  await page.goto('/')

  // --- real login via console-mailer PIN ---
  await page.getByPlaceholder('you@example.com').fill('me@example.com')
  await page.getByRole('button', { name: 'Send code' }).click()
  await expect(page.getByPlaceholder('6-digit PIN')).toBeVisible()

  const pin = waitForPin()
  await page.getByPlaceholder('6-digit PIN').fill(pin)
  await page.getByRole('button', { name: 'Log in' }).click()
  await expect(page.getByText('Welcome, me@example.com')).toBeVisible()

  if (!(await engineUp(page))) {
    test.skip(true, 'Docker engine unavailable — full-stack terminal test skipped')
    return
  }

  try {
    // --- clean slate: the stack data dir persists between runs; leftovers
    // would make row-scoping below ambiguous ---
    await deleteAllProjects(page)

    // --- create through the REAL form, exactly as a user would ---
    await page.getByPlaceholder(/Repo URL/).fill('')
    await page.getByRole('button', { name: 'Create project' }).click()
    // create is synchronous (sandbox up before the response); done when the
    // button comes back
    await expect(page.getByRole('button', { name: 'Create project' })).toBeEnabled({
      timeout: 60_000,
    })
    const terminalButtons = page.getByRole('button', { name: 'Terminal' })
    await expect(terminalButtons).toHaveCount(1)

    // --- open the terminal: this must work with NO session ever created ---
    await terminalButtons.click()
    await expect(page.locator('.xterm-screen')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('Connected')).toBeVisible({ timeout: 10_000 })

    await page.keyboard.type('echo journey7\n')
    await expect
      .poll(async () => page.locator('.xterm-rows').innerText(), { timeout: 15_000 })
      .toContain('journey7')

    // plumbing proof in the backend's own audit trail: clicking Terminal
    // created the session AND attached to it — no API help needed
    const log = readFileSync('test-results/sps-stack-data/events.log', 'utf8')
    expect(log).toContain('"session.create"')
    expect(log).toContain('"terminal.attach"')
  } finally {
    // destructor: never leak containers/volumes, even on failure
    await deleteAllProjects(page)
  }
})

async function engineUp(page: Page): Promise<boolean> {
  const res = await page.request.get('/api/projects')
  return res.status() !== 500
}

// The server maps an unreachable engine to 500 on project ops; a healthy
// engine answers 200 even with zero projects.
async function deleteAllProjects(page: Page): Promise<void> {
  const res = await page.request.get('/api/projects')
  if (!res.ok()) return
  for (const p of ((await res.json()) as { projects: { id: string }[] }).projects) {
    await page.request.delete(`/api/projects/${p.id}?scope=all`)
  }
}

function waitForPin(): string {
  const logPath = 'test-results/sps-stack-server.log'
  for (let i = 0; i < 50; i++) {
    try {
      const m = readFileSync(logPath, 'utf8').match(/login PIN for me@example\.com: (\d{6})/)
      if (m) return m[1]
    } catch {
      // log not flushed yet
    }
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 100)
  }
  throw new Error(`no PIN found in ${logPath} — does the console mailer print it?`)
}
