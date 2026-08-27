import type { Page } from "@playwright/test"
import analyticsSummary from "../src/test/contracts/analytics_summary.json" with { type: "json" }
import apiKeyAliasTargets from "../src/test/contracts/api_key_alias_targets_page.json" with { type: "json" }
import usageIdentities from "../src/test/contracts/usage_identities_page.json" with { type: "json" }

export const statusPayload = {
  running: true,
  sync_running: false,
  last_status: "completed",
  last_run_at: "2026-05-18T09:30:00Z",
  timezone: "Asia/Shanghai",
  version: "e2e",
}

export const usageOverviewPayload = {
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

export const usageEvents = Array.from({ length: 11 }, (_, index) => ({
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

export const authFileIdentitiesPayload = {
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

export interface RecordedAPIRequest {
  path: string
  method: string
  url: URL
}

export interface MockAPIOptions {
  /** /auth/session 返回值；默认 true。传 false 可走登录流程。 */
  authenticated?: boolean
  /** 覆盖 /analytics/core 响应；可基于请求 URL 返回不同数据。 */
  analyticsCore?: (url: URL) => Record<string, unknown>
  /** 每次 API 请求回调，用于断言请求参数。 */
  onRequest?: (request: RecordedAPIRequest) => void
}

export async function installMockAPI(page: Page, options: MockAPIOptions = {}) {
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const apiIndex = url.pathname.indexOf("/api/v1")
    const path = apiIndex >= 0 ? url.pathname.slice(apiIndex + "/api/v1".length) : url.pathname
    const method = request.method()
    options.onRequest?.({ path, method, url })

    if (path === "/auth/session") {
      await route.fulfill({ json: { authenticated: options.authenticated ?? true } })
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
      if (options.analyticsCore) {
        await route.fulfill({ json: options.analyticsCore(url) })
        return
      }
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
      if (![1, 10, 20, 50, 100, 500, 1000].includes(pageSize)) {
        await route.fulfill({ status: 400, json: { error: `invalid page_size ${pageSize}` } })
        return
      }
      const start = (page - 1) * pageSize
      await route.fulfill({ json: {
        events: usageEvents.slice(start, start + pageSize),
        total_count: usageEvents.length,
        page,
        page_size: pageSize,
        total_pages: Math.max(1, Math.ceil(usageEvents.length / pageSize)),
      } })
      return
    }
    if (path === "/usage/identities/page") {
      const payload = url.searchParams.get("auth_type") === "1" ? authFileIdentitiesPayload : usageIdentities
      const pageSize = Number(url.searchParams.get("page_size") ?? "10")
      await route.fulfill({ json: { ...payload, page_size: pageSize, total_pages: Math.max(1, Math.ceil(payload.total_count / pageSize)) } })
      return
    }
    if (path === "/usage/api-keys/page") {
      const pageSize = Number(url.searchParams.get("page_size") ?? "100")
      await route.fulfill({ json: { ...apiKeyAliasTargets, page_size: pageSize, total_pages: Math.max(1, Math.ceil(apiKeyAliasTargets.total_count / pageSize)) } })
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
