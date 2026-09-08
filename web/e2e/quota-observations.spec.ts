import { expect, test } from "@playwright/test"
import { authFileIdentitiesPayload, installMockAPI } from "./mock-api"

// Synthetic observations exercise the real page/hooks across failed and successful
// refreshes. Backend persistence and restart behavior are covered by Go tests.
for (const theme of ["light", "dark"]) {
  test(`${theme} quota observations survive age, refresh failure and page reload`, async ({ page }, testInfo) => {
    await page.clock.setFixedTime(new Date("2026-09-08T04:45:00Z"))
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    await installMockAPI(page)

    const baseIdentity = {
      ...authFileIdentitiesPayload.identities[0],
      alias: "",
      status: "active",
      unavailable: false,
      metadata_observed_at: "2026-09-08T04:44:00Z",
      next_retry_after: null,
      passive_quota: null,
      passive_model_quotas: [],
      active_start: null,
      active_until: null,
    }
    const identities = [
      { ...baseIdentity, id: 601, identity: "manual-observation", name: "Manual account", displayName: "Manual account" },
      {
        ...baseIdentity, id: 602, identity: "reported-observation", name: "Reported account", displayName: "Reported account",
        passive_quota: {
          source: "cpa_passive", scope: "account", observed_at: "2026-09-08T04:04:00Z",
          quota: [{ key: "weekly", label: "Weekly", usedPercent: 10, window: { seconds: 604_800 } }],
        },
      },
      { ...baseIdentity, id: 603, identity: "never-observed", name: "No observation account", displayName: "No observation account" },
    ]
    await page.route(/\/api\/v1\/usage\/identities\/page(?:\?|$)/, (route) => route.fulfill({
      json: { identities, total_count: identities.length, page: 1, page_size: 100, total_pages: 1 },
    }))

    let observation = {
      id: "manual-observation", observedAt: "2026-09-08T00:00:00Z",
      quota: [{ key: "primary", label: "5h", usedPercent: 25, window: { seconds: 18_000 } }],
    }
    let observationReads = 0
    let refreshCalls = 0
    await page.route(/\/api\/v1\/quota\/observations$/, (route) => {
      observationReads++
      return route.fulfill({ json: { items: [observation] } })
    })
    await page.route(/\/api\/v1\/quota\/refresh$/, (route) => {
      refreshCalls++
      return route.fulfill({ json: {
        tasks: [{ authIndex: "manual-observation", taskId: `refresh-${refreshCalls}` }],
        rejected: [], accepted: 1, skipped: 0, limit: 20,
      } })
    })
    await page.route(/\/api\/v1\/quota\/refresh\/refresh-\d+$/, (route) => {
      const failed = route.request().url().endsWith("refresh-1")
      if (!failed) observation = {
        ...observation, observedAt: "2026-09-08T04:45:00Z",
        quota: [{ key: "primary", label: "5h", usedPercent: 30, window: { seconds: 18_000 } }],
      }
      return route.fulfill({ json: {
        taskId: failed ? "refresh-1" : "refresh-2", authIndex: "manual-observation",
        status: failed ? "failed" : "completed",
        ...(failed ? { error: "HTTP 503" } : { quota: observation }),
      } })
    })
    await page.goto("/")
    const capacity = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: /^Live Capacity/ }) })
    const cardFor = (name: string) => capacity.locator(".group").filter({ has: page.getByRole("button", { name: `Refresh ${name}`, exact: true }) })
    const manual = cardFor("Manual account")
    const reported = cardFor("Reported account")
    const never = cardFor("No observation account")
    await expect(manual.getByText("25% used", { exact: true })).toBeVisible()
    await expect(manual.getByText("Last updated 4h ago", { exact: true })).toBeVisible()
    await expect(reported.getByText("Last updated 41m ago", { exact: true })).toBeVisible()
    await expect(never.getByText("No reading", { exact: true })).toHaveCount(2)
    await expect(never.getByText(/^Last updated /)).toHaveCount(0)
    await expect(capacity.getByText(/^(Stale|Cache expires|Expired)$/)).toHaveCount(0)
    expect(refreshCalls).toBe(0)

    await manual.getByRole("button", { name: "Refresh Manual account", exact: true }).click()
    await expect(manual.locator("span[title^='Refresh failed:']")).toBeVisible()
    await expect(manual.getByText("25% used", { exact: true })).toBeVisible()
    await expect(manual.getByText("Last updated 4h ago", { exact: true })).toBeVisible()
    await capacity.screenshot({ path: testInfo.outputPath(`quota-observations-${theme}.png`), animations: "disabled" })

    const readsBeforeReload = observationReads
    await page.reload()
    await expect(manual.getByText("Last updated 4h ago", { exact: true })).toBeVisible()
    await expect(manual.getByText("25% used", { exact: true })).toBeVisible()
    expect(observationReads).toBeGreaterThan(readsBeforeReload)
    expect(refreshCalls).toBe(1)

    await manual.getByRole("button", { name: "Refresh Manual account", exact: true }).click()
    await expect(manual.getByText("30% used", { exact: true })).toBeVisible()
    await expect(manual.getByText("Last updated just now", { exact: true })).toBeVisible()
    await page.reload()
    await expect(manual.getByText("30% used", { exact: true })).toBeVisible()
    await expect(manual.getByText("Last updated just now", { exact: true })).toBeVisible()
    expect(refreshCalls).toBe(2)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  })
}
