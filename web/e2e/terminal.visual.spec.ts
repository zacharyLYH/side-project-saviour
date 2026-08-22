import { expect, test, type Page } from '@playwright/test'

// Visual tests for the terminal: the app runs against network-level mocks
// (no backend), and the WebSocket is scripted with page.routeWebSocket so a
// real xterm.js instance renders deterministic frames. Snapshots assert the
// rendered pixels — this is what catches renderer/layout regressions.
// Phone is covered deliberately: it is the primary use case (PRD litmus).

async function scriptMocks(page: Page) {
  await page.route('**/api/auth/me', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body:      JSON.stringify({ email: 'me@example.com' }) }))
  await page.route('**/api/projects', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ projects: [{ id: 'abc123', name: 'demo' }] }) }))
  // TerminalView ensures the session exists before dialing the WS
  await page.route('**/api/projects/abc123/sessions', (route) =>
    route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ name: 'main' }) }))

  // script the WS the page opens when Terminal is clicked: echo a prompt,
  // then reply to input like a tiny line-buffered fake shell. Input arrives
  // as one frame per keystroke, so commands are only recognized on Enter.
  await page.routeWebSocket(/\/ws\/projects\/abc123/, (ws) => {
    let line = ''
    ws.send(JSON.stringify({ type: 'output', data: 'sps sandbox\r\n$ ' }))
    ws.onMessage((message) => {
      const frame = JSON.parse(String(message)) as { type: string; data?: string }
      if (frame.type !== 'input' || !frame.data) return
      line += frame.data
      ws.send(JSON.stringify({ type: 'output', data: frame.data.replace(/\r/g, '\r\n') }))
      if (!line.includes('\r')) return
      const entered = line
      line = ''
      if (entered.trimEnd().endsWith('exit')) {
        ws.send(JSON.stringify({ type: 'exit', code: 0 }))
      }
    })
  })
}

async function typeUntilRendered(page: Page) {
  await expect(page.locator('.xterm-screen')).toBeVisible()
  await expect(page.getByText('Connected')).toBeVisible()

  await page.keyboard.type('echo hello')
  // the fake shell echoes back; wait until it shows in the accessibility rows
  await expect.poll(async () => {
    const text = await page.locator('.xterm-rows').innerText()
    // xterm rows may break mid-word; strip row-newlines to get logical text
    return text.replace(/\n/g, '')
  }).toContain('echo hello')
}

test.describe('desktop', () => {
  test.use({ viewport: { width: 1280, height: 720 } })

  test('terminal renders on desktop', async ({ page }) => {
    await scriptMocks(page)
    await page.goto('/')
    await page.getByRole('button', { name: 'Terminal' }).click()
    await typeUntilRendered(page)

    await expect(page).toHaveScreenshot('terminal-desktop.png', { maxDiffPixelRatio: 0.02 })

    await page.keyboard.press('Enter')
    await page.keyboard.type('exit')
    await page.keyboard.press('Enter')
    await expect(page.getByText('Disconnected')).toBeVisible({ timeout: 5_000 })
  })
})

test.describe('phone', () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true })

  test('terminal renders on a phone', async ({ page }) => {
    await scriptMocks(page)
    await page.goto('/')
    await page.getByRole('button', { name: 'Terminal' }).click()
    await typeUntilRendered(page)

    await expect(page).toHaveScreenshot('terminal-phone.png', { maxDiffPixelRatio: 0.02 })
  })
})
