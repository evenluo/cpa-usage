import { expect, test } from "@playwright/test"
import { installMockAPI } from "./mock-api"

function buildBreakdown(prefix: string) {
  return {
    label: `${prefix} Agent`,
    alias: `${prefix} Agent`,
    traceability: "sk-live-…abcd",
    identity: "sk-live-…abcd",
    auth_type: 1,
    auth_type_name: "oauth",
    type: "claude",
    provider: "Claude",
    is_deleted: false,
    total_cost: 0.3,
    total_tokens: 50_000,
    canonical_valid_attempts: 4,
    request_count: 4,
    success_count: 4,
    failure_count: 0,
    success_rate: 1,
    last_used_at: "2026-05-18T09:30:00Z",
    cost_available: true,
    cost_status: "available",
    trend: [],
  }
}

/** 按请求参数返回不同聚合结果，让 e2e 能断言参数真正传导到 API 并驱动 UI。 */
function analyticsCoreFor(url: URL): Record<string, unknown> {
  const range = url.searchParams.get("range") ?? "7d"
  const granularity = url.searchParams.get("granularity") ?? "hour"
  const provider = url.searchParams.get("provider") ?? ""
  const pointCount = range === "30d" && granularity === "day" ? 3 : 7
  return {
    granularity,
    summary: {
      total_cost: range === "30d" ? 1.25 : 0.5,
      total_tokens: 100_000 + pointCount,
      request_count: pointCount,
      success_count: pointCount - 1,
      failure_count: 1,
      input_tokens: 50_000,
      output_tokens: 40_000,
      reasoning_tokens: 5_000,
      cache_read_tokens: 200,
      cache_write_tokens: 0,
      success_rate: 0.96,
      cost_available: true,
      cost_status: "available",
      cache_read_share: 0.4,
      cache_read_coverage: 100,
      cache_read_share_state: "available",
      accounting: {
        total_attempts: pointCount,
        valid_attempts: pointCount,
        coverage_pct: 100,
        states: { absent: 0, valid: pointCount },
        valid_quality: { complete: pointCount, inconsistent: 0, unclassified: 0 },
        composition: {
          total_tokens: 100_000 + pointCount,
          input: { total_tokens: 50_000, uncached_tokens: 49_800, cache_read_tokens: 200, cache_write_tokens: 0 },
          output: { total_tokens: 40_000, non_reasoning_tokens: 35_000, reasoning_tokens: 5_000 },
          unclassified_tokens: 10_000 + pointCount,
        },
      },
    },
    trend: Array.from({ length: pointCount }, (_, index) => ({
      label: `2026-05-${String(11 + index).padStart(2, "0")}`,
      total_cost: 0.1,
      total_tokens: 10_000,
      input_tokens: 5_000,
      output_tokens: 4_000,
      reasoning_tokens: 500,
      cache_read_tokens: 500,
      canonical_valid_attempts: 1,
      request_count: 1,
      success_count: 1,
      failure_count: 0,
      cost_available: true,
      cost_status: "available",
    })),
    provider_options: [
      { provider: "claude", request_count: 5, total_tokens: 60_000, canonical_valid_attempts: 5, total_cost: 0.3, cost_available: true, cost_status: "available" },
      { provider: "codex", request_count: 3, total_tokens: 40_000, canonical_valid_attempts: 3, total_cost: 0.2, cost_available: true, cost_status: "available" },
    ],
    key_alias_breakdown: [buildBreakdown("Research")],
    api_key_breakdown: [buildBreakdown("sk-live")],
    model_distribution: [{
      model: `${provider || "all"}-model`,
      provider: provider || "all",
      total_cost: 0.5,
      total_tokens: 100_000,
      input_tokens: 50_000,
      output_tokens: 40_000,
      reasoning_tokens: 5_000,
      cache_read_tokens: 4_000,
      canonical_valid_attempts: pointCount,
      cache_read_share: 10,
      cache_read_coverage: 80,
      cache_read_share_state: "partial",
      request_count: pointCount,
      success_count: pointCount - 1,
      failure_count: 1,
      success_rate: 96,
      total_latency_ms: 100,
      latency_sample_count: 1,
      average_latency_ms: 100,
      cost_available: true,
      cost_status: "available",
    }],
    insights: [{
      type: "metric_completeness",
      severity: "green",
      title: `${provider || "All providers"} metrics complete`,
      detail: "All model costs are available.",
      subject: provider || "All providers",
      metric_label: "Metric Completeness",
      metric_value: 1,
      count: 1,
      cost_status: "available",
    }],
    ...(provider ? { provider_options: [] } : {}),
  }
}

test("signed-out users are redirected to login and can sign in to reach the dashboard", async ({ page }) => {
  await installMockAPI(page, { authenticated: false, analyticsCore: analyticsCoreFor })

  await page.goto("/")
  await page.waitForURL(/\/login$/)
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible()

  await page.getByPlaceholder("Enter password").fill("test-password")
  await page.getByRole("button", { name: "Sign in" }).click()

  await page.waitForURL(/\/$/)
  await expect(page.getByRole("heading", { name: "Intelligence" })).toBeVisible()
  await expect(page.getByRole("heading", { name: "Sign in" })).toHaveCount(0)
})

test("dashboard renders account capacity, diagnostics, and analysis layers", async ({ page }) => {
  await installMockAPI(page, { analyticsCore: analyticsCoreFor })

  await page.goto("/")
  await expect(page.getByRole("heading", { name: "Intelligence" })).toBeVisible()

  for (const kpi of ["Cost", "Tokens", "Attempts", "Success", "Cache"]) {
    // 首次冷启动时 3 个视口 project 并行加载，KPI 渲染可能慢于默认 5s 超时。
    await expect(page.getByText(kpi, { exact: true }).first()).toBeVisible({ timeout: 10_000 })
  }
  await expect(page.getByText("Trends")).toBeVisible()
  await expect(page.getByText("Key Leaderboard")).toBeVisible()
  await expect(page.getByText("sk-live Agent")).toBeVisible()
  await expect(page.getByText("Activity Heatmap")).toBeVisible()
  await expect(page.getByText("Attempt Health")).toBeVisible()
  await expect(page.getByText("Request evidence")).toBeVisible()
  await expect(page.getByText("Live Capacity")).toBeVisible()
  await expect(page.getByText("Model Mix")).toBeVisible()
  await expect(page.getByText("all-model")).toBeVisible()
  // 信息层次契约：选样窗口分析 → 24h 性能与健康 → 当前容量与长期活跃。
  const layerTop = async (pattern: RegExp | string) => {
    const box = await page.getByText(pattern, { exact: typeof pattern === "string" }).first().boundingBox()
    return box?.y ?? Number.POSITIVE_INFINITY
  }
  const analysisTop = await layerTop("Cost")
  const trendTop = await layerTop("Trends")
  const liveCapacityTop = await layerTop("Live Capacity")
  const diagnosticsTop = await layerTop("Performance & health")
  expect(analysisTop).toBeLessThan(trendTop)
  expect(trendTop).toBeLessThan(diagnosticsTop)
  expect(diagnosticsTop).toBeLessThan(liveCapacityTop)
  expect(liveCapacityTop).toBeLessThan(await layerTop("Activity Heatmap"))
  expect(await layerTop("Attempt performance")).toBeLessThan(await layerTop("Attempt Health"))
  await expect(page.getByText("Needs attention", { exact: true })).toHaveCount(0)
  await expect(page.getByText("All providers metrics complete")).toHaveCount(0)
})

test("switching time range and granularity changes the analytics request and the visible KPIs", async ({ page }) => {
  const analyticsRequests: URL[] = []
  await installMockAPI(page, {
    analyticsCore: analyticsCoreFor,
    onRequest: (request) => {
      if (request.path === "/analytics/core") analyticsRequests.push(request.url)
    },
  })

  await page.goto("/")
  await expect(page.getByText("Trends")).toBeVisible()

  const lastRange = () => analyticsRequests.at(-1)?.searchParams.get("range")
  const lastGranularity = () => analyticsRequests.at(-1)?.searchParams.get("granularity")
  await expect.poll(lastRange).toBe("7d")
  await expect.poll(lastGranularity).toBe("hour")

  await page.getByRole("button", { name: "30 days" }).click()
  await expect.poll(lastRange).toBe("30d")
  await expect.poll(lastGranularity).toBe("day")
  await expect(page.getByTestId("kpi-value-cost")).toHaveText("$1.25")
  await expect(page.getByTestId("kpi-value-attempts")).toHaveText("3")

  await page.getByRole("button", { name: "Hour", exact: true }).click()
  await expect.poll(lastGranularity).toBe("hour")
  await expect(page.getByTestId("kpi-value-attempts")).toHaveText("7")
})

test("provider filter scopes the analytics request and manual sync reports completion", async ({ page }) => {
  const analyticsRequests: URL[] = []
  const evidenceRequests: URL[] = []
  let syncRequests = 0
  await installMockAPI(page, {
    analyticsCore: analyticsCoreFor,
    onRequest: (request) => {
      if (request.path === "/analytics/core") analyticsRequests.push(request.url)
      if (request.path === "/usage/events") evidenceRequests.push(request.url)
      if (request.path === "/sync" && request.method === "POST") syncRequests += 1
    },
  })

  await page.goto("/")
  await expect(page.getByText("Trends")).toBeVisible()

  await page.getByRole("button", { name: /^claude/ }).click()
  await expect.poll(() => analyticsRequests.at(-1)?.searchParams.get("provider")).toBe("claude")
  await expect(page.getByText("claude-model")).toBeVisible()
  await expect(page.getByText("claude metrics complete")).toHaveCount(0)

  await page.getByRole("link", { name: "View all attempts" }).click()
  await expect(page).toHaveURL(/\/requests\?/)
  const requestsURL = new URL(page.url())
  expect(requestsURL.pathname).toBe("/requests")
  expect(requestsURL.searchParams.get("provider")).toBe("claude")
  expect(requestsURL.searchParams.get("model")).toBe("")
  expect(requestsURL.searchParams.get("result")).toBe("")
  await expect(page.getByTestId("request-provider-scope")).toHaveText("Provider: claude")
  await expect.poll(() => evidenceRequests.at(-1)?.searchParams.get("provider")).toBe("claude")
  await expect(page.getByText("claude-evidence-model").first()).toBeVisible()

  await page.goto("/operations")
  await expect(page.getByText("Operational Status")).toBeVisible()
  await page.getByRole("button", { name: "Trigger Sync" }).click()
  await expect(page.getByText("Sync triggered")).toBeVisible()
  await expect.poll(() => syncRequests).toBe(1)
})
