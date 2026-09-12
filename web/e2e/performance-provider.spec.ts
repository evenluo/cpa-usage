import { expect, test } from "@playwright/test"
import { installMockAPI, usageAttemptPerformance, type RecordedAPIRequest } from "./mock-api"
import analytics from "../src/test/contracts/analytics_summary.json" with { type: "json" }

for (const theme of ["light", "dark"]) {
  test(`${theme} performance provider defaults locally and never relabels stale results`, async ({ page }, testInfo) => {
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    const requests: RecordedAPIRequest[] = []
    const providers = Array.from({ length: 9 }, (_, index) => ({ ...analytics.provider_options[0], provider: `provider-${index}`, request_count: index + 1 }))
    await installMockAPI(page, { onRequest: (request) => requests.push(request) })
    await page.route(/\/api\/v1\/usage\/performance\/providers(?:\?|$)/, (route) => route.fulfill({
      json: {
        window_start: analytics.range_start,
        window_end: analytics.range_end,
        provider_options: providers,
      },
    }))
    let releaseResponse: () => void = () => {}
    const pendingResponse = new Promise<void>((resolve) => { releaseResponse = resolve })
    await page.route(/\/api\/v1\/usage\/performance(?:\?|$)/, async (route) => {
      const provider = new URL(route.request().url()).searchParams.get("provider")
      if (provider === "provider-0") await pendingResponse
      const row = { ...usageAttemptPerformance.providers.items[0], value: `${provider}-model`, label: `${provider}-model` }
      await route.fulfill({ json: { ...usageAttemptPerformance, ...row, models: { items: [row], other_count: 0 } } })
    })
    await page.goto("/")
    const card = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: "Attempt performance", exact: true }) })
    const selector = card.getByRole("combobox", { name: "Performance provider" })
    // Complete the independent, viewport-triggered capacity reads before
    // asserting that changing the performance provider makes no global requests.
    const capacity = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: /^Live Capacity/ }) })
    await capacity.scrollIntoViewIfNeeded()
    await expect.poll(() => requests.filter((request) => request.path === "/usage/identities/page").length).toBe(1)
    await expect.poll(() => requests.filter((request) => request.path === "/quota/observations").length).toBe(1)
    await selector.scrollIntoViewIfNeeded()
    await expect(selector).toHaveText("provider-8")
    await selector.click()
    await expect(page.getByRole("option")).toHaveCount(9)
    await page.keyboard.press("Escape")
    await expect(card.getByRole("img", { name: /provider-8-model/ })).toBeVisible()
    await card.getByRole("button", { name: "Output TPS", exact: true }).click()
    await expect(card.getByRole("img", { name: /provider-8-model/ })).toBeVisible()
    await expect(card.getByText("Select a provider.")).toHaveCount(0)
    await card.screenshot({ animations: "disabled", path: testInfo.outputPath(`alignment-${theme}.png`) })
    await selector.click()
    await page.screenshot({ animations: "disabled", path: testInfo.outputPath(`provider-menu-${theme}.png`) })
    await page.keyboard.press("Escape")
    await expect(selector).toBeFocused()
    const globalRequests = requests.filter((request) => !request.path.startsWith("/usage/performance")).length
    await selector.click()
    await page.getByRole("option", { name: "provider-0", exact: true }).click()
    await expect(selector).toHaveText("provider-0")
    await expect(card.getByRole("img", { name: /provider-8-model/ })).toHaveCount(0)
    releaseResponse()
    await expect(card.getByRole("img", { name: /provider-0-model/ })).toBeVisible()
    expect(requests.filter((request) => !request.path.startsWith("/usage/performance"))).toHaveLength(globalRequests)
    await card.getByRole("button", { name: "Latency", exact: true }).click()
    await expect(card.getByRole("link", { name: /Inspect provider-0-model success attempts/ })).toHaveAttribute("href", /provider=provider-0/)
    await card.screenshot({ animations: "disabled", path: testInfo.outputPath(`provider-selector-${theme}.png`) })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  })
}
