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

const usageEvents = Array.from({ length: 11 }, (_, index) => ({
    id: index + 1,
    timestamp: `2026-05-18T09:${String(index).padStart(2, "0")}:00Z`,
    model: "mobile-overflow-regression-model-with-extra-long-provider-suffix",
    source: "sk-live-mobile-overflow-regression-key-display-with-extra-long-suffix",
    auth_index: "sk-live-mobile-overflow-regression-key-display-with-extra-long-suffix",
    api_key_alias: "Agent API Key With A Very Long Mobile Label",
    api_key_display: "sk-live-mobile-overflow-regression-key-display-with-extra-long-suffix",
    failed: index === 2,
    latency_ms: index === 0 ? 21_245 : 240 + index,
    ttft_ms: index === 0 ? 1_052 : null,
    output_tps: index === 0 ? 48.33358094488189 : null,
    tokens: {
      output_tokens: index === 0 ? 976 : 0,
      total_tokens: index === 0 ? 105_091 : 1_700_000_000 + index,
    },
  }))

const authFileIdentitiesPayload = {
  identities: [
    {
      id: 501,
      name: "Codex Auth",
      displayName: "Codex Auth",
      alias: "Agent Codex",
      auth_type: 1,
      auth_type_name: "oauth",
      identity: "codex-auth-e2e",
      type: "codex",
      provider: "Codex",
      total_tokens: 0,
      total_cost: 0,
      cost_available: false,
      last_used_at: null,
    },
    {
      id: 502,
      name: "Unsupported OpenAI",
      displayName: "Unsupported OpenAI",
      alias: "",
      auth_type: 1,
      auth_type_name: "oauth",
      identity: "openai-auth-e2e",
      type: "openai",
      provider: "OpenAI",
      total_tokens: 0,
      total_cost: 0,
      cost_available: false,
      last_used_at: null,
    },
  ],
  total_count: 2,
  page: 1,
  page_size: 100,
  total_pages: 1,
}

const quotaCachePayload = {
  items: [
    {
      id: "codex-auth-e2e",
      quota: [
        { key: "rate_limit.primary_window", label: "5h", usedPercent: 35, resetAfterSeconds: 3600, planType: "plus" },
        { key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 62, resetAfterSeconds: 7200, planType: "plus" },
      ],
    },
  ],
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

const { comparison: _comparison, heatmap, previous_range_start: _previousRangeStart, previous_range_end: _previousRangeEnd, ...dashboardAnalyticsCore } = dashboardAnalyticsSummary

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
  await page.addInitScript(() => {
    localStorage.setItem("cpa-theme", "dark")
  })
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
  await expect(page.getByText("Live Capacity")).toBeVisible()
  await expect(page.getByText("Agent Codex")).toBeVisible()
  await expect(page.getByText("Plus", { exact: true })).toBeVisible()
  await expect(page.getByText("cached", { exact: true })).toBeVisible()
  await expect(page.getByText("Unsupported OpenAI")).toBeVisible()
  await expect(page.getByText("OA", { exact: true })).toBeVisible()
  const codexLogoWell = page.locator('[aria-label="Codex"]')
  await expect(codexLogoWell).toHaveCount(1)
  await expect
    .poll(async () => codexLogoWell.evaluate((element) => getComputedStyle(element).backgroundColor))
    .toContain("255, 255, 255")
  await expect(page.getByText("Request Evidence")).toBeVisible()
  await expect(page.getByText("Agent API Key").first()).toBeVisible()
  const evidenceCard = page.getByText("Request Evidence").locator("xpath=ancestor::*[contains(@class,'rounded-xl')][1]")
  await expect(evidenceCard.getByText("Output TPS", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("48.3 tok/s", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("Latency", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("21.25s", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("Tokens", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("105.09K", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByRole("button", { name: /Show request evidence/ })).toHaveCount(0)
  await expectFixedOverviewCardHeights(page)
  await evidenceCard.getByRole("link", { name: "View all requests" }).click()
  await expect(page).toHaveURL(/\/requests$/)
  await expect(page.getByRole("heading", { name: "Request Evidence" })).toBeVisible()
  await expect(page.getByText("Page 1 of 2", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: "Select request 2" }).click()
  const selectedRequest = page.getByRole("region", { name: "Selected request" })
  await expect(selectedRequest.getByText("Output TPS", { exact: true }).locator("..").getByText("-", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: "Next page" }).click()
  await expect(page.getByText("Page 2 of 2", { exact: true })).toBeVisible()
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
    if (path === "/analytics/core") {
      await route.fulfill({ json: dashboardAnalyticsCore })
      return
    }
    if (path === "/analytics/heatmap") {
      await route.fulfill({ json: { ...dashboardAnalyticsCore, heatmap } })
      return
    }
    if (path === "/usage/overview") {
      await route.fulfill({ json: usageOverviewPayload })
      return
    }
    if (path === "/usage/request-health") {
      await route.fulfill({ json: usageOverviewPayload })
      return
    }
    if (path === "/usage/events") {
      const page = Number(url.searchParams.get("page") ?? "1")
      const pageSize = Number(url.searchParams.get("page_size") ?? "100")
      const start = (page - 1) * pageSize
      await route.fulfill({ json: {
        events: usageEvents.slice(start, start + pageSize),
        total_count: usageEvents.length,
        page,
        page_size: pageSize,
        total_pages: Math.ceil(usageEvents.length / pageSize),
      } })
      return
    }
    if (path === "/usage/identities/page") {
      await route.fulfill({ json: url.searchParams.get("auth_type") === "1" ? authFileIdentitiesPayload : usageIdentities })
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
    if (path === "/quota/cache" && method === "POST") {
      await route.fulfill({ json: quotaCachePayload })
      return
    }
    if (path === "/quota/refresh" && method === "POST") {
      await route.fulfill({ json: { tasks: [], rejected: [], accepted: 0, skipped: 0, limit: 20 } })
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
