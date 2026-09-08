import { expect, test } from "@playwright/test"
import { installMockAPI, usageAttemptPerformance, type RecordedAPIRequest } from "./mock-api"
import analytics from "../src/test/contracts/analytics_summary.json" with { type: "json" }

for (const theme of ["light", "dark"]) {
  test(`${theme} performance provider defaults locally and never relabels stale results`, async ({ page }, testInfo) => {
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    const requests: RecordedAPIRequest[] = []
    const providers = Array.from({ length: 9 }, (_, index) => ({ ...analytics.provider_options[0], provider: `provider-${index}`, request_count: index + 1 }))
    await installMockAPI(page, { onRequest: (request) => requests.push(request) })
    await page.route(/\/api\/v1\/analytics\/core(?:\?|$)/, (route) => route.fulfill({ json: { ...analytics, provider_options: providers } }))
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
    await expect(selector).toHaveValue("provider-8")
    await expect(selector.locator("option")).toHaveCount(9)
    await expect(card.getByRole("img", { name: /provider-8-model/ })).toBeVisible()
    await card.getByRole("button", { name: "Output TPS", exact: true }).click()
    await expect(card.getByRole("img", { name: /provider-8-model/ })).toBeVisible()
    await expect(card.getByText("Select a provider to compare output speed.")).toHaveCount(0)
    const globalRequests = requests.filter((request) => request.path !== "/usage/performance").length
    await selector.selectOption("provider-0")
    await expect(selector).toHaveValue("provider-0")
    await expect(card.getByRole("img", { name: /provider-8-model/ })).toHaveCount(0)
    releaseResponse()
    await expect(card.getByRole("img", { name: /provider-0-model/ })).toBeVisible()
    expect(requests.filter((request) => request.path !== "/usage/performance")).toHaveLength(globalRequests)
    await card.getByRole("button", { name: "Successful latency", exact: true }).click()
    await expect(card.getByRole("link", { name: /Inspect provider-0-model success attempts/ })).toHaveAttribute("href", /provider=provider-0/)
    await card.screenshot({ animations: "disabled", path: testInfo.outputPath(`provider-selector-${theme}.png`) })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  })
}
