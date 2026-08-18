import { expect, test } from '@playwright/test'

// API calls are mocked at the network layer (page.route): the component and
// its fetch wiring run for real, but no Go server is booted. Mocks mirror the
// real response shapes.
function mockJSON(route: { fulfill: (opts: object) => Promise<void> }, status: number, body: unknown, headers?: object) {
  return route.fulfill({
    status,
    contentType: 'application/json',
    headers,
    body: JSON.stringify(body),
  })
}

test('shows the login form when logged out', async ({ page }) => {
  await page.route('**/api/auth/me', (route) => mockJSON(route, 401, { error: 'unauthorized' }))
  await page.goto('/')

  await expect(page.getByPlaceholder('you@example.com')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Send code' })).toBeVisible()
})

test('logs in with a PIN and shows the logged-in view', async ({ page }) => {
  await page.route('**/api/auth/me', (route) => mockJSON(route, 401, { error: 'unauthorized' }))
  await page.route('**/api/auth/request-pin', (route) => mockJSON(route, 200, { ok: true }))
  await page.route('**/api/auth/verify', (route) =>
    mockJSON(route, 200, { email: 'me@example.com' }, { 'set-cookie': 'sps_session=testtoken; Path=/; HttpOnly; SameSite=Strict' }),
  )
  await page.route('**/api/ping', (route) => mockJSON(route, 200, { pong: 'true', version: 'dev' }))
  await page.goto('/')

  await page.getByPlaceholder('you@example.com').fill('me@example.com')
  await page.getByRole('button', { name: 'Send code' }).click()
  await expect(page.getByPlaceholder('6-digit PIN')).toBeVisible()

  await page.getByPlaceholder('6-digit PIN').fill('123456')
  await page.getByRole('button', { name: 'Log in' }).click()

  await expect(page.getByText('Welcome, me@example.com')).toBeVisible()
  await expect(page.getByText(/Server ping: \{"pong":"true"/)).toBeVisible()
})

test('shows the ping error when logged in and logs out', async ({ page }) => {
  await page.route('**/api/auth/me', (route) => mockJSON(route, 200, { email: 'me@example.com' }))
  await page.route('**/api/ping', (route) => route.fulfill({ status: 500, body: 'boom' }))
  await page.route('**/api/auth/logout', (route) => mockJSON(route, 200, { ok: true }))
  await page.goto('/')

  await expect(page.getByText('Welcome, me@example.com')).toBeVisible()
  await expect(page.getByText('Server ping: HTTP 500')).toBeVisible()

  await page.getByRole('button', { name: 'Log out' }).click()
  await expect(page.getByPlaceholder('you@example.com')).toBeVisible()
})
