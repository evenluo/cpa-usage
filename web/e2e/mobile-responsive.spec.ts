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
    columns: 8,
    bucket_seconds: 180,
    window_start: "2026-05-18T00:00:00Z",
    window_end: "2026-05-19T00:00:00Z",
    block_details: Array.from({ length: 8 }, (_, index) => ({
      start_time: `2026-05-18T0${index}:00:00Z`,
      end_time: `2026-05-18T0${index}:03:00Z`,
      success: index === 2 ? 0 : 3,
      failure: index === 2 ? 1 : 0,
      rate: index === 2 ? 0 : 1,
    })),
  },
}

const usageEventsPayload = {
  events: [
    {
      id: 1,
      timestamp: "2026-05-18T09:00:00Z",
      model: "priced-model",
      source: "sk-l************alue",
      auth_index: "sk-l************alue",
      failed: false,
      latency_ms: 240,
      tokens: { total_tokens: 1700 },
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
  await expect(page.getByText("Key Leaderboard")).toBeVisible()
  await expect(page.getByText("Request Evidence")).toBeVisible()
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
      await route.fulfill({ json: analyticsSummary })
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
