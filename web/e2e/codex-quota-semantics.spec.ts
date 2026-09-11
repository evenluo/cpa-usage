import { expect, test } from "@playwright/test"
import { authFileIdentitiesPayload, installMockAPI } from "./mock-api"

// Synthetic fixtures keep account allowances separate from request-model
// snapshots. Missing Reserve readings do not establish account eligibility.
for (const theme of ["light", "dark"]) {
  test(`${theme} Codex cards show Reserve directly without model quota copies`, async ({ page }, testInfo) => {
    await page.clock.setFixedTime(new Date("2026-09-11T07:30:00Z"))
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    await installMockAPI(page)

    const resetAt = "2026-09-15T01:28:58Z"
    const observedAt = "2026-09-11T07:27:00Z"
    const requestObservedAt = "2026-09-10T10:22:00Z"
    const withReserve = {
      ...authFileIdentitiesPayload.identities[0],
      id: 701, identity: "codex-with-reserve", name: "Codex with reserve", displayName: "Codex with reserve", alias: "",
      provider: "codex", type: "codex", plan_type: "pro", status: "active", disabled: false, unavailable: false,
      metadata_observed_at: observedAt, active_start: null, active_until: null, next_retry_after: null,
      passive_quota: {
        source: "cpa_passive", scope: "account", active_limit: "premium", observed_at: observedAt,
        quota: [{ key: "codex.rate_limit.primary", label: "Weekly", usedPercent: 86, resetAt, window: { seconds: 604_800 } }],
      },
      passive_model_quotas: [{
        source: "cpa_passive", scope: "model", model: "gpt-5.6-terra", active_limit: "premium", observed_at: requestObservedAt,
        quota: [{ key: "codex.rate_limit.primary", label: "Weekly", usedPercent: 34, resetAt, window: { seconds: 604_800 } }],
      }],
    }
    const withoutReserve = {
      ...withReserve,
      id: 702, identity: "codex-without-reserve", name: "Codex without reserve", displayName: "Codex without reserve",
      metadata_observed_at: "2026-09-11T07:20:00Z",
      passive_quota: {
        ...withReserve.passive_quota,
        observed_at: "2026-09-11T07:20:00Z",
        quota: [{ key: "codex.rate_limit.primary", label: "Weekly", usedPercent: 48, resetAt, window: { seconds: 604_800 } }],
      },
      passive_model_quotas: [{
        ...withReserve.passive_model_quotas[0], observed_at: "2026-09-11T07:29:00Z",
        quota: [{ key: "codex.additional-gpt-reserve.primary", label: "gpt-reserve Weekly", usedPercent: 12, resetAt, window: { seconds: 604_800 } }],
      }],
    }
    await page.route(/\/api\/v1\/usage\/identities\/page(?:\?|$)/, (route) => route.fulfill({
      json: { identities: [withReserve, withoutReserve], total_count: 2, page: 1, page_size: 100, total_pages: 1 },
    }))
    await page.route(/\/api\/v1\/quota\/observations$/, (route) => route.fulfill({ json: { items: [{
      id: withReserve.identity, observedAt,
      quota: [{ key: "additional_rate_limits.gpt-reserve.primary_window", label: "gpt-reserve Weekly", metric: "base_model_inference", usedPercent: 0, resetAt, window: { seconds: 604_800 } }],
    }] } }))
    let refreshCalls = 0
    page.on("request", (request) => {
      if (/\/api\/v1\/quota\/(refresh|check)$/.test(request.url())) refreshCalls++
    })

    await page.goto("/")
    const capacity = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: /^Live Capacity/ }) })
    await capacity.scrollIntoViewIfNeeded()
    const card = capacity.locator(".group").filter({ has: page.getByRole("button", { name: "Refresh Codex with reserve", exact: true }) })
    await expect(card.getByLabel("Weekly: 86% used", { exact: true })).toBeVisible()
    await expect(card.getByText("Last updated 3m ago", { exact: true })).toBeVisible()
    const reserve = card.getByRole("region", { name: "Luna Reserve", exact: true })
    await expect(reserve).toBeVisible()
    await expect(reserve.getByLabel("Luna Reserve Weekly: 0% used", { exact: true })).toBeVisible()
    await expect(reserve.getByText(/gpt-reserve/)).toBeVisible()
    expect(await reserve.evaluate((element) => element.closest("details"))).toBeNull()
    await card.locator("summary").filter({ hasText: /more$/ }).click()
    await expect(card.getByRole("region", { name: "Model request observations", exact: true })).toHaveCount(0)
    await expect(card.getByText("gpt-5.6-terra", { exact: true })).toHaveCount(0)
    await expect(card.getByLabel("Weekly: 34% used", { exact: true })).toHaveCount(0)
    await card.locator("summary").filter({ hasText: /more$/ }).click()
    await card.evaluate((element) => element.scrollIntoView({ block: "center" }))
    await page.screenshot({ path: testInfo.outputPath(`codex-reserve-present-${theme}.png`), animations: "disabled" })

    const missingCard = capacity.locator(".group").filter({ has: page.getByRole("button", { name: "Refresh Codex without reserve", exact: true }) })
    await missingCard.evaluate((element) => element.scrollIntoView({ block: "center" }))
    const missingReserve = missingCard.getByRole("region", { name: "Luna Reserve", exact: true })
    await expect(missingReserve.getByText("No reading", { exact: true })).toBeVisible()
    await expect(missingReserve.getByText(/gpt-reserve/)).toBeVisible()
    await expect(missingReserve.getByText("No Reserve reading has been collected. Availability is unknown.", { exact: true })).toBeVisible()
    await expect(missingReserve.locator("[aria-label*='% used']")).toHaveCount(0)
    await expect(missingCard.getByText("Last updated 10m ago", { exact: true })).toBeVisible()
    await expect(missingCard.getByLabel("Weekly: 48% used", { exact: true })).toBeVisible()
    await expect(missingCard.getByText("gpt-5.6-terra", { exact: true })).toHaveCount(0)
    await page.screenshot({ path: testInfo.outputPath(`codex-reserve-missing-${theme}.png`), animations: "disabled" })
    expect(refreshCalls).toBe(0)
    expect(await card.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    expect(await missingCard.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true)
  })
}
