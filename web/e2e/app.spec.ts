import { expect, test } from '@playwright/test'

// API calls are mocked at the network layer (page.route): the component and
// its fetch wiring run for real, but no Go server is booted. The mock mirrors
// the real /api/ping response shape.
test('placeholder page shows the server ping', async ({ page }) => {
  await page.route('**/api/ping', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ pong: 'true', version: 'dev' }),
    }),
  )
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'Side Project Saviour' })).toBeVisible()
  await expect(page.getByText(/server ping: \{"pong":"true"/i)).toBeVisible()
})

test('shows the error when the API call fails', async ({ page }) => {
  await page.route('**/api/ping', (route) => route.fulfill({ status: 500, body: 'boom' }))
  await page.goto('/')

  await expect(page.getByText('Server ping: HTTP 500')).toBeVisible()
})
