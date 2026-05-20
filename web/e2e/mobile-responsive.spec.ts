import { expect, type Page, test } from "@playwright/test"
import analyticsSummary from "../src/test/contracts/analytics_summary.json" with { type: "json" }
import apiKeyAliasTargets from "../src/test/contracts/api_key_alias_targets_page.json" with { type: "json" }
import usageIdentities from "../src/test/contracts/usage_identities_page.json" with { type: "json" }

const statusPayload = {
  running: true,
  sync_running: false,
  last_status: "completed",
  last_run_at: "2026-05-18T09:30:00Z",
  timezone: "Asia/Shanghai",
  version: "e2e",
}

const usageOverviewPayload = {
  service_health: {
    total_success: 18,
    total_failure: 1,
    success_rate: 94.7,
    rows: 1,
    columns: 480,
    bucket_seconds: 180,
    window_start: "2026-05-18T00:00:00Z",
    window_end: "2026-05-19T00:00:00Z",
    block_details: Array.from({ length: 480 }, (_, index) => ({
      start_time: new Date(Date.UTC(2026, 4, 18, 0, index * 3)).toISOString(),
      end_time: new Date(Date.UTC(2026, 4, 18, 0, index * 3 + 3)).toISOString(),
      success: index === 2 ? 0 : 3,
      failure: index === 2 ? 1 : 0,
      rate: index === 2 ? 0 : 1,
    })),
  },
}

const usageEventsPayload = {
  events: Array.from({ length: 6 }, (_, index) => ({
    id: index + 1,
    timestamp: `2026-05-18T09:0${index}:00Z`,
    model: "mobile-overflow-regression-model-with-extra-long-provider-suffix",
    source: "sk-live-mobile-overflow-regression-key-display-with-extra-long-suffix",
    auth_index: "sk-live-mobile-overflow-regression-key-display-with-extra-long-suffix",
    api_key_alias: "Agent API Key With A Very Long Mobile Label",
    api_key_display: "sk-live-mobile-overflow-regression-key-display-with-extra-long-suffix",
    failed: index === 2,
    latency_ms: 240 + index,
    tokens: { total_tokens: 1_700_000_000 + index },
  })),
}

const pricingPayload = {
  pricing: [
    {
      model: "priced-model",
      prompt_price_per_1m: 1,
      completion_price_per_1m: 2,
      cache_price_per_1m: 0.5,
    },
  ],
}

const usedModelsPayload = {
  models: ["priced-model", "mobile-overflow-regression-model"],
}

const dashboardAnalyticsSummary = {
  ...analyticsSummary,
  trend: [
    {
      ...analyticsSummary.trend[0],
      label: "2026-05-11",
      bucket_start: "2026-05-11T00:00:00Z",
      bucket_end: "2026-05-12T00:00:00Z",
      total_tokens: 1600000,
      input_tokens: 1000000,
      output_tokens: 500000,
      reasoning_tokens: 100000,
      cached_tokens: 100000,
      request_count: 1,
    },
    {
      ...analyticsSummary.trend[0],
      label: "2026-05-12",
      bucket_start: "2026-05-12T00:00:00Z",
      bucket_end: "2026-05-13T00:00:00Z",
      total_cost: 0.35,
      total_tokens: 320000,
      input_tokens: 220000,
      output_tokens: 80000,
      reasoning_tokens: 10000,
      cached_tokens: 10000,
      request_count: 1,
    },
    {
      ...analyticsSummary.trend[0],
      label: "2026-05-13",
      bucket_start: "2026-05-13T00:00:00Z",
      bucket_end: "2026-05-14T00:00:00Z",
      total_cost: 0.15,
      total_tokens: 380100,
      input_tokens: 280100,
      output_tokens: 20000,
      reasoning_tokens: 0,
      cached_tokens: 50000,
      request_count: 1,
    },
  ],
}

test.beforeEach(async ({ page }) => {
  await mockAPI(page)
})

test("mobile uses bottom navigation without the fixed desktop sidebar", async ({ page }) => {
  await page.goto("/")
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()

  const usesMobileNav = await page.evaluate(() => window.matchMedia("(max-width: 767px)").matches)
  if (usesMobileNav) {
    await expect(page.getByLabel("Desktop navigation")).toBeHidden()
    await expect(page.getByLabel("Mobile navigation")).toBeVisible()
    await expectMobileNavigationPinnedToViewportBottom(page)
  } else {
    await expect(page.getByLabel("Desktop navigation")).toBeVisible()
    await expect(page.getByLabel("Mobile navigation")).toBeHidden()
  }

  await expectNoDocumentOverflow(page)
  await page.getByRole("link", { name: /Reference/ }).click()
  await expect(page.getByRole("heading", { name: "Reference" })).toBeVisible()
  await expectNoDocumentOverflow(page)
  await page.getByRole("link", { name: /Operations/ }).click()
  await expect(page.getByRole("heading", { name: "Operations" })).toBeVisible()
  await expectNoDocumentOverflow(page)
})

test("dashboard controls and evidence stay inside each responsive viewport", async ({ page }) => {
  await page.goto("/")

  await page.getByRole("button", { name: "30 days" }).click()
  await page.getByRole("button", { name: "Day", exact: true }).click()
  await page.getByRole("button", { name: "Trend view: Tokens" }).click()

  await expect(page.getByText("Trend Workbench")).toBeVisible()
  const chartLegend = page.locator(".recharts-legend-wrapper")
  await expect(chartLegend.getByText("Tokens", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Input", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Output", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Reasoning", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Cached", { exact: true })).toBeVisible()
  await expect(page.getByText("Key Leaderboard")).toBeVisible()
  await expect(page.getByText("Request Evidence")).toBeVisible()
  await expect(page.getByText("Agent API Key").first()).toBeVisible()
  await expectFixedOverviewCardHeights(page)
  await expectNoDocumentOverflow(page)
})

test("reference data controls are usable without viewport overflow", async ({ page }) => {
  await page.goto("/reference")

  await page.getByPlaceholder("Search alias or key...").fill("Agent")
  await page.getByRole("button", { name: "Key alias scope: Accounts" }).click()
  await page.getByRole("button", { name: "Key alias scope: API Keys" }).click()
  await page.getByRole("button", { name: /Edit alias for Agent API Key/ }).click()
  await page.locator('input[name^="key-alias-"]').last().fill("Agent API Key Mobile")
  await page.getByRole("button", { name: /Save alias for Agent API Key/ }).click()

  await expect(page.getByText("Alias saved")).toBeVisible()
  await expectNoDocumentOverflow(page)
})

test("login and operations remain usable on small screens", async ({ page }) => {
  await page.goto("/login")
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible()
  await expectNoDocumentOverflow(page)

  await page.goto("/operations")
  await expect(page.getByText("Operational Status")).toBeVisible()
  await page.getByRole("button", { name: "Trigger Sync" }).click()
  await expect(page.getByText("Sync triggered")).toBeVisible()
  await expectNoDocumentOverflow(page)
})

async function mockAPI(page: Page) {
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const apiIndex = url.pathname.indexOf("/api/v1")
    const path = apiIndex >= 0 ? url.pathname.slice(apiIndex + "/api/v1".length) : url.pathname
    const method = request.method()

    if (path === "/auth/session") {
      await route.fulfill({ json: { authenticated: true } })
      return
    }
    if (path === "/auth/login" && method === "POST") {
      await route.fulfill({ json: { authenticated: true } })
      return
    }
    if (path === "/status") {
      await route.fulfill({ json: statusPayload })
      return
    }
    if (path === "/analytics/summary") {
      await route.fulfill({ json: dashboardAnalyticsSummary })
      return
    }
    if (path === "/usage/overview") {
      await route.fulfill({ json: usageOverviewPayload })
      return
    }
    if (path === "/usage/events") {
      await route.fulfill({ json: usageEventsPayload })
      return
    }
    if (path === "/usage/identities/page") {
      await route.fulfill({ json: usageIdentities })
      return
    }
    if (path === "/usage/api-keys/page") {
      await route.fulfill({ json: apiKeyAliasTargets })
      return
    }
    if (path === "/pricing" && method === "GET") {
      await route.fulfill({ json: pricingPayload })
      return
    }
    if (path === "/pricing" && method === "PUT") {
      await route.fulfill({ json: pricingPayload.pricing[0] })
      return
    }
    if (path === "/models/used") {
      await route.fulfill({ json: usedModelsPayload })
      return
    }
    if (path.startsWith("/usage/identities/") && path.endsWith("/alias")) {
      await route.fulfill({ json: { alias: "Agent Research" } })
      return
    }
    if (path.startsWith("/usage/api-keys/") && path.endsWith("/alias")) {
      await route.fulfill({ json: { alias: "Agent API Key" } })
      return
    }
    if (path === "/sync" && method === "POST") {
      await route.fulfill({ json: statusPayload })
      return
    }

    await route.fulfill({ status: 404, body: `Unhandled API route: ${method} ${path}` })
  })
}

async function expectNoDocumentOverflow(page: Page) {
  await expect
    .poll(async () =>
      page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }))
    )
    .toEqual(expect.objectContaining({ scrollWidth: expect.any(Number) }))

  const overflow = await page.evaluate(() => {
    const root = document.documentElement
    return root.scrollWidth - root.clientWidth
  })
  expect(overflow).toBeLessThanOrEqual(1)
}

async function expectMobileNavigationPinnedToViewportBottom(page: Page) {
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight))

  const navPosition = await page.getByLabel("Mobile navigation").evaluate((node) => {
    const rect = node.getBoundingClientRect()
    const style = window.getComputedStyle(node)
    return {
      bottomGap: Math.abs(window.innerHeight - rect.bottom),
      position: style.position,
      transform: style.transform,
      willChange: style.willChange,
    }
  })

  expect(navPosition.position).toBe("fixed")
  expect(navPosition.bottomGap).toBeLessThanOrEqual(1)
  expect(navPosition.transform).toBe("none")
  expect(navPosition.willChange).toBe("auto")
}

async function expectFixedOverviewCardHeights(page: Page) {
  const heights = await page.evaluate(() => {
    const healthHeading = Array.from(document.querySelectorAll("h3")).find((node) => node.textContent?.includes("Request Health"))
    const evidenceHeading = Array.from(document.querySelectorAll("h3")).find((node) => node.textContent?.includes("Request Evidence"))
    const healthCard = healthHeading?.closest(".rounded-xl")
    const evidenceCard = evidenceHeading?.closest(".rounded-xl")
    return {
      isWide: window.matchMedia("(min-width: 1280px)").matches,
      health: healthCard?.getBoundingClientRect().height ?? 0,
      evidence: evidenceCard?.getBoundingClientRect().height ?? 0,
    }
  })

  if (!heights.isWide) return
  expect(Math.abs(heights.health - heights.evidence)).toBeLessThanOrEqual(1)
  expect(heights.evidence).toBeLessThanOrEqual(330)
}
