import { expect, test } from "@playwright/test"
import { authFileIdentitiesPayload, installMockAPI } from "./mock-api"

// Synthetic contract fixture reproduces a newer account snapshot and an older
// Terra request snapshot. It does not assert reserve eligibility for real users.
for (const theme of ["light", "dark"]) {
  test(`${theme} Codex limits distinguish reserve from model request observations`, async ({ page }, testInfo) => {
    await page.clock.setFixedTime(new Date("2026-09-11T07:30:00Z"))
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    await installMockAPI(page)

    const resetAt = "2026-09-15T01:28:58Z"
    const observedAt = "2026-09-11T07:27:00Z"
    const requestObservedAt = "2026-09-10T10:22:00Z"
    const account = {
      ...authFileIdentitiesPayload.identities[0],
      id: 701, identity: "codex-quota-semantics", name: "Codex account", displayName: "Codex account", alias: "",
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
    await page.route(/\/api\/v1\/usage\/identities\/page(?:\?|$)/, (route) => route.fulfill({
      json: { identities: [account], total_count: 1, page: 1, page_size: 100, total_pages: 1 },
    }))
    await page.route(/\/api\/v1\/quota\/observations$/, (route) => route.fulfill({ json: { items: [{
      id: account.identity, observedAt,
      quota: [{ key: "additional_rate_limits.gpt-reserve.secondary_window", label: "gpt-reserve Weekly", metric: "base_model_inference", usedPercent: 12, resetAt, window: { seconds: 604_800 } }],
    }] } }))
    let refreshCalls = 0
    page.on("request", (request) => {
      if (/\/api\/v1\/quota\/(refresh|check)$/.test(request.url())) refreshCalls++
    })

    await page.goto("/")
    const capacity = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: /^Live Capacity/ }) })
    await capacity.scrollIntoViewIfNeeded()
    const card = capacity.locator(".group").filter({ has: page.getByRole("button", { name: "Refresh Codex account", exact: true }) })
    await expect(card.getByLabel("Weekly: 86% used", { exact: true })).toBeVisible()
    await expect(card.getByText("Last updated 3m ago", { exact: true })).toBeVisible()
    await expect(card.getByText("Model request observations (1)", { exact: true })).not.toBeVisible()
    await card.locator("summary").filter({ hasText: /more$/ }).click()
    await expect(card.getByLabel("Luna Reserve Weekly: 12% used", { exact: true })).toBeVisible()
    await expect(card.getByText("Extra Luna usage after regular usage is exhausted.", { exact: true })).toBeVisible()
    const modelObservation = card.getByRole("region", { name: "gpt-5.6-terra request observation", exact: true })
    await expect(modelObservation.getByText("gpt-5.6-terra", { exact: true })).toBeVisible()
    await expect(modelObservation.getByLabel("Weekly: 34% used", { exact: true })).toBeVisible()
    await expect(modelObservation.locator(`time[datetime="${requestObservedAt}"]`)).toHaveText(/^Observed /)
    await expect(card.getByText("Account quota snapshots observed during model requests. Readings can be older than the account limits above.", { exact: true })).toBeVisible()
    await expect(card.getByText(/Per-model quotas/)).toHaveCount(0)
    expect(refreshCalls).toBe(0)
    expect(await card.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    // The account list has its own scroll container. Capture both positions at
    // the tested viewport so clipping cannot hide the request observation time.
    await card.getByLabel("Weekly: 86% used", { exact: true }).evaluate((element) => element.scrollIntoView({ block: "center" }))
    await page.screenshot({ path: testInfo.outputPath(`codex-quota-account-${theme}.png`), animations: "disabled" })
    await modelObservation.evaluate((element) => element.scrollIntoView({ block: "center" }))
    await expect(modelObservation).toBeInViewport({ ratio: 1 })
    await page.screenshot({ path: testInfo.outputPath(`codex-quota-observations-${theme}.png`), animations: "disabled" })
  })
}
