import { expect, type Page, test } from "@playwright/test"
import { dashboardAnalyticsCore, installMockAPI, type RecordedAPIRequest } from "./mock-api"

const correlationWindowEnd = "2026-09-07T11:00:00.123456789Z"

const canonicalAccounting = {
  total_attempts: 10,
  valid_attempts: 3,
  coverage_pct: 30,
  states: {
    absent: 1,
    malformed: 1,
    unsupported_accounting_version: 1,
    unsupported_schema_version: 1,
    missing: 1,
    unknown_quality: 1,
    invalid: 1,
    valid: 3,
  },
  valid_quality: { complete: 1, inconsistent: 1, unclassified: 1 },
  composition: {
    total_tokens: 155,
    input: { total_tokens: 100, uncached_tokens: 70, cache_read_tokens: 20, cache_write_tokens: 10 },
    output: { total_tokens: 50, non_reasoning_tokens: 40, reasoning_tokens: 10 },
    unclassified_tokens: 5,
  },
}

test("fixed 24-hour diagnostics open the matching request evidence selection", async ({ page }, testInfo) => {
  await setTheme(page, "light")
  const requests: RecordedAPIRequest[] = []
  await installMockAPI(page, {
    analyticsCore: () => ({
      ...dashboardAnalyticsCore,
      summary: { ...dashboardAnalyticsCore.summary, accounting: canonicalAccounting },
    }),
    onRequest: (request) => requests.push(request),
  })

  await page.goto("/")
  await expect(page.getByText("Failure concentration")).toBeVisible()
  await expect(page.getByText("Attempt performance")).toBeVisible()
  await expect(page.getByText("Observed model mappings")).toBeVisible()
  await page.getByText("Canonical token composition").click()
  await expect(page.getByText(/3 \/ 10 attempts have valid canonical structure/)).toBeVisible()
  await expect(page.getByText(/complete 1, inconsistent 1, unclassified 1/)).toBeVisible()
  await expect(page.getByText(/Absent \/ historical 1; Malformed fields 1/)).toContainText("Invalid bucket totals 1")
  await expect(page.getByText(/Cost completeness: partial/)).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath("dashboard-diagnostics.png"), fullPage: true })

  await page.getByRole("link", { name: "Inspect HTTP 429 failures" }).click()
  await expectEvidenceSelection(page, requests, {
    windowEnd: "2026-09-07T12:00:00Z",
    status: "429",
    result: "failed",
  })

  await page.goto("/")
  await page.getByRole("link", { name: "Inspect successful attempts at or above p95 latency" }).click()
  await expectEvidenceSelection(page, requests, {
    windowEnd: "2026-09-07T12:00:00.123456789Z",
    minLatencyMS: "9000",
    result: "success",
  })

  await page.goto("/")
  await page.getByRole("link", { name: "Inspect route-a to actual-a attempts" }).click()
  await expectEvidenceSelection(page, requests, {
    windowEnd: "2026-09-07T12:00:00Z",
    provider: "provider-a",
    model: "actual-a",
    modelAlias: "route-a",
  })
})

test("correlated attempts keep the fixed scope and clear filters that hide siblings", async ({ page }) => {
  await setTheme(page, "light")
  const requests: RecordedAPIRequest[] = []
  await installMockAPI(page, { onRequest: (request) => requests.push(request) })

  await page.goto("/requests?provider=claude&model=sonnet&modelAlias=route-a&account=auth-1&endpoint=%2Fv1%2Fmessages&status=429&minLatencyMS=9000&windowEnd=2026-09-07T11%3A00%3A00.123456789Z&result=failed")
  await expect(page.getByRole("heading", { name: "Request Evidence" })).toBeVisible()
  await page.getByRole("button", { name: "View correlated attempts" }).click()

  const correlationScope = page.locator('[aria-label="Correlation scope"]')
  await expect(correlationScope).toContainText("Correlated attempts are distinct observed rows")
  await expect(correlationScope).toContainText("other providers are not included")
  await expect(correlationScope.getByText(/retry count|final outcome/i)).toHaveCount(0)

  await expect.poll(() => {
    const url = new URL(page.url())
    return {
      provider: url.searchParams.get("provider"),
      requestId: url.searchParams.get("requestId"),
      windowEnd: url.searchParams.get("windowEnd"),
      hiddenFilters: ["model", "modelAlias", "account", "endpoint", "status", "minLatencyMS", "result"]
        .filter((key) => Boolean(url.searchParams.get(key))),
    }
  }).toEqual({
    provider: "claude",
    requestId: "request-correlated-e2e",
    windowEnd: correlationWindowEnd,
    hiddenFilters: [],
  })

  await expect(page.getByText("CSV export includes the full frozen selection, up to 5,000 matching requests.")).toBeVisible()
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "Download CSV" }).click(),
  ])
  expect(download.suggestedFilename()).toBe("request-evidence.csv")
  await expect.poll(() => {
    const url = latestRequestURL(requests, "/usage/events/export")
    return url ? {
      range: url.searchParams.get("range"),
      provider: url.searchParams.get("provider"),
      requestId: url.searchParams.get("request_id"),
      windowEnd: url.searchParams.get("window_end"),
      hiddenFilters: ["model", "model_alias", "account", "endpoint", "status", "min_latency_ms", "result"]
        .filter((key) => Boolean(url.searchParams.get(key))),
    } : null
  }).toEqual({
    range: "24h",
    provider: "claude",
    requestId: "request-correlated-e2e",
    windowEnd: correlationWindowEnd,
    hiddenFilters: [],
  })

  await expect.poll(() => {
    const url = latestRequestURL(requests, "/usage/events")
    return url ? {
      provider: url.searchParams.get("provider"),
      requestId: url.searchParams.get("request_id"),
      windowEnd: url.searchParams.get("window_end"),
      hiddenFilters: ["model", "model_alias", "account", "endpoint", "status", "min_latency_ms", "result"]
        .filter((key) => Boolean(url.searchParams.get(key))),
    } : null
  }).toEqual({
    provider: "claude",
    requestId: "request-correlated-e2e",
    windowEnd: correlationWindowEnd,
    hiddenFilters: [],
  })
})

test("dark responsive Live Capacity separates stored evidence and loads model support explicitly", async ({ page }, testInfo) => {
  await setTheme(page, "dark")
  const requests: RecordedAPIRequest[] = []
  await installMockAPI(page, { onRequest: (request) => requests.push(request) })

  await page.goto("/")
  await expect(page.locator("html")).toHaveClass(/dark/)
  await expect(page.getByText("Agent Codex")).toBeVisible()
  await expect(page.getByText("Temporarily unavailable", { exact: true }).first()).toBeVisible()
  await expect(page.getByText("Operator disabled", { exact: true })).toBeVisible()
  await expect(page.getByText(/Retry eligibility is not a recovery guarantee/).first()).toBeVisible()

  const passive = page.getByRole("group", { name: "CPA passive quota observation" })
  await expect(passive).toContainText("Latest provider watermark observed by CPA")
  await expect(passive.locator("time[datetime='2026-09-07T08:00:00Z']")).toBeVisible()
  await expect(passive.locator("time[datetime='2026-09-07T07:30:00Z']")).toBeVisible()
  await expect(page.getByText("Manual capacity probe")).toBeVisible()
  const timing = page.getByRole("group", { name: "Account and cache timing" }).first()
  await expect(timing).toContainText("Capacity probe evidence")
  await expect(timing.locator("time[datetime='2026-08-31T09:05:00Z']")).toBeVisible()
  await expect(timing.locator("time[datetime='2026-08-31T09:25:00Z']")).toBeVisible()

  expect(requests.filter((request) => request.path === "/quota/refresh")).toHaveLength(0)
  expect(requests.filter((request) => request.path === "/usage/identities/model-support")).toHaveLength(0)
  expect(requests.filter((request) => request.path === "/quota/cache")).toHaveLength(1)

  await page.getByRole("button", { name: "Select displayed" }).click()
  await page.getByRole("button", { name: "Load model support" }).click()
  await expect(page.getByText("Partial selected scope")).toBeVisible()
  await expect(page.getByText("Failed accounts are unknown, not unsupported")).toBeVisible()
  await expect(page.getByText("observed in 1/1 loaded accounts")).toBeVisible()

  await expect.poll(() => requests.filter((request) => request.path === "/usage/identities/model-support").length).toBe(1)
  const supportRequest = requests.find((request) => request.path === "/usage/identities/model-support")
  expect(supportRequest?.method).toBe("POST")
  expect(JSON.parse(supportRequest?.body || "null")).toEqual({ identity_ids: [501, 502] })
  expect(requests.filter((request) => request.path === "/quota/refresh")).toHaveLength(0)
  await expectNoDocumentOverflow(page)
  await page.screenshot({ path: testInfo.outputPath("live-capacity-dark.png"), fullPage: true })
})

async function expectEvidenceSelection(
  page: Page,
  requests: RecordedAPIRequest[],
  expected: {
    windowEnd: string
    provider?: string
    model?: string
    modelAlias?: string
    status?: string
    minLatencyMS?: string
    result?: string
  },
) {
  await expect(page.getByRole("heading", { name: "Request Evidence" })).toBeVisible()
  await expect.poll(() => {
    const pageURL = new URL(page.url())
    const requestURL = latestRequestURL(requests, "/usage/events")
    return requestURL ? {
      pageWindowEnd: pageURL.searchParams.get("windowEnd"),
      requestWindowEnd: requestURL.searchParams.get("window_end"),
      provider: requestURL.searchParams.get("provider") || undefined,
      model: requestURL.searchParams.get("model") || undefined,
      modelAlias: requestURL.searchParams.get("model_alias") || undefined,
      status: requestURL.searchParams.get("status") || undefined,
      minLatencyMS: requestURL.searchParams.get("min_latency_ms") || undefined,
      result: requestURL.searchParams.get("result") || undefined,
    } : null
  }).toEqual({
    pageWindowEnd: expected.windowEnd,
    requestWindowEnd: expected.windowEnd,
    provider: expected.provider,
    model: expected.model,
    modelAlias: expected.modelAlias,
    status: expected.status,
    minLatencyMS: expected.minLatencyMS,
    result: expected.result,
  })
}

function latestRequestURL(requests: RecordedAPIRequest[], path: string): URL | undefined {
  return requests.filter((request) => request.path === path).at(-1)?.url
}

async function setTheme(page: Page, theme: "light" | "dark") {
  await page.addInitScript((selectedTheme) => {
    localStorage.setItem("cpa-theme", selectedTheme)
  }, theme)
}

async function expectNoDocumentOverflow(page: Page) {
  await expect.poll(async () => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1)
}
