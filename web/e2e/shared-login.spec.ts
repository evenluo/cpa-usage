import { expect, test } from "@playwright/test"
import { installMockAPI } from "./mock-api"

test("remembered CPA v2 login opens Usage without signing in again", async ({ page }) => {
  const managementKey = "e2e-shared-management-key"
  await page.addInitScript((key) => {
    const payload = JSON.stringify({
      state: { apiBase: window.location.origin, managementKey: key, rememberPassword: true },
      version: 0,
    })
    const mask = new TextEncoder().encode(`cli-proxy-api-webui::secure-storage|${window.location.host}`)
    const bytes = new TextEncoder().encode(payload).map((byte, index) => byte ^ mask[index % mask.length])
    localStorage.setItem("cli-proxy-auth", `enc::v2::${btoa(String.fromCharCode(...bytes))}`)
  }, managementKey)
  await installMockAPI(page)
  let sessionBearer: string | undefined
  let loginRequests = 0
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    const bearer = request.headers().authorization
    if (path.endsWith("/auth/session")) {
      sessionBearer = bearer
      await route.fulfill({ json: { authenticated: bearer === `Bearer ${managementKey}` } })
      return
    }
    if (path.endsWith("/auth/login")) loginRequests += 1
    if (bearer !== `Bearer ${managementKey}`) {
      await route.fulfill({ status: 401, json: { error: "unauthorized" } })
      return
    }
    await route.fallback()
  })

  await page.goto("/")

  await expect.poll(() => sessionBearer).toBe(`Bearer ${managementKey}`)
  await expect(page.getByRole("heading", { name: "Intelligence" })).toBeVisible()
  await expect(page.getByRole("heading", { name: "Sign in", exact: true })).toHaveCount(0)
  expect(loginRequests).toBe(0)
})
