import type { Page } from "@playwright/test"
import analyticsSummary from "../src/test/contracts/analytics_summary.json" with { type: "json" }
import apiKeyAliasTargets from "../src/test/contracts/api_key_alias_targets_page.json" with { type: "json" }
import usageFailureDistribution from "../src/test/contracts/usage_failure_distribution.json" with { type: "json" }
import usageIdentities from "../src/test/contracts/usage_identities_page.json" with { type: "json" }
import usageModelMappings from "../src/test/contracts/usage_model_mappings.json" with { type: "json" }

export const statusPayload = {
  running: true,
  sync_running: false,
  last_status: "completed",
  last_run_at: "2026-05-18T09:30:00Z",
  timezone: "Asia/Shanghai",
  version: "e2e",
}

export const metricsPayload = {
  uptime_seconds: 3_600,
  poller_running: true,
  poller_sync_running: false,
  redis_inbox_pending: 2,
  redis_events_processed_total: 25,
  redis_events_processed_batches_total: 2,
  redis_events_last_processed_at: "2026-09-07T01:02:03Z",
  redis_events_processing_rate_per_minute: 12.5,
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
    request_id: index < 2 ? "request-correlated-e2e" : `request-${index + 1}`,
    failed: index === 2,
    latency_ms: index === 0 ? 21_245 : 240 + index,
    ttft_ms: index === 0 ? 1_052 : null,
    output_tps: index === 0 ? 48.33358094488189 : null,
    tokens: {
      input_tokens: index === 0 ? 104_115 : null,
      output_tokens: index === 0 ? 976 : null,
      reasoning_tokens: index === 0 ? 100 : null,
      cache_read_tokens: index === 0 ? 20 : null,
      cache_write_tokens: index === 0 ? 0 : null,
      unclassified_tokens: index === 0 ? 0 : null,
      total_tokens: index === 0 ? 105_091 : null,
    },
    attempt_facts: {
      generate: index === 0 ? true : null,
      stream: index === 0 ? true : null,
      request_service_tier: null,
      response_service_tier: null,
      output_tps: index === 0 ? 48.33358094488189 : null,
      accounting: index === 0 ? {
        state: "valid",
        accounting_version: 2,
        schema_version: 2,
        quality: "complete",
        total_tokens: 105_091,
        input: { total_tokens: 104_115, uncached_tokens: 104_095, cache_read_tokens: 20, cache_write_tokens: 0 },
        output: { total_tokens: 976, non_reasoning_tokens: 876, reasoning_tokens: 100 },
        unclassified_tokens: 0,
      } : {
        state: "absent",
        accounting_version: null,
        schema_version: null,
        quality: null,
        total_tokens: null,
        input: { total_tokens: null, uncached_tokens: null, cache_read_tokens: null, cache_write_tokens: null },
        output: { total_tokens: null, non_reasoning_tokens: null, reasoning_tokens: null },
        unclassified_tokens: null,
      },
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
      canonical_valid_attempts: 0,
      total_cost: 0,
      cost_available: false,
      last_used_at: null,
      status: "error",
      unavailable: true,
      metadata_observed_at: "2026-09-07T08:00:00Z",
      next_retry_after: "2026-09-07T08:30:00Z",
      passive_quota: {
        source: "cpa_passive",
        scope: "account",
        observed_at: "2026-09-07T08:00:00Z",
        active_limit: "codex_primary",
        quota: [{ key: "primary", label: "5h", usedPercent: 25, allowed: false, resetAfterSeconds: 120, window: { seconds: 18_000 } }],
      },
      passive_model_quotas: [{
        source: "cpa_passive",
        scope: "model",
        model: "gpt-exact",
        observed_at: "2026-09-07T07:30:00Z",
        quota: [{ key: "weekly", label: "Weekly", usedPercent: 40, window: { seconds: 604_800 } }],
      }],
      active_start: "2026-08-21T02:59:00Z",
      active_until: "2026-09-25T07:15:00Z",
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
      canonical_valid_attempts: 0,
      total_cost: 0,
      cost_available: false,
      last_used_at: null,
      disabled: true,
      status: "disabled",
      unavailable: true,
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
      cachedAt: "2026-08-31T09:05:00Z",
      expiresAt: "2026-08-31T09:25:00Z",
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

function percentile(populationCount: number, sampleCount: number, p50: number | null, p95: number | null) {
  return {
    population_count: populationCount,
    sample_count: sampleCount,
    coverage: populationCount === 0 ? null : sampleCount / populationCount,
    p50,
    p95,
  }
}

const performanceSummary = {
  successful_attempts: 18,
  failed_attempts: 2,
  successful_execution: { generating_streaming: 10, non_generating: 2, non_streaming: 1, unknown: 5 },
  latency_ms: {
    successful: percentile(18, 18, 500, 9_000),
    failed: percentile(2, 1, 12_000, 12_000),
  },
  ttft_ms: {
    generating_streaming: percentile(10, 8, 120, 1_500),
    unknown_execution: percentile(5, 0, null, null),
  },
  output_tps: {
    generating_streaming: percentile(10, 7, 42, 88),
  },
}

const performanceSummaryWithoutComparableTPS = {
  ...performanceSummary,
  output_tps: {
    generating_streaming: percentile(10, 0, null, null),
  },
}

export const usageAttemptPerformance = {
  ...performanceSummaryWithoutComparableTPS,
  window_start: "2026-09-06T12:00:00Z",
  window_end: "2026-09-07T12:00:00.123456789Z",
  total_attempts: 20,
  providers: { items: [{ ...performanceSummary, value: "claude", label: "claude", attempt_count: 20 }], other_count: 0 },
  models: { items: [{ ...performanceSummaryWithoutComparableTPS, value: "sonnet", label: "sonnet", attempt_count: 20 }], other_count: 3 },
  accounts: { items: [{ ...performanceSummaryWithoutComparableTPS, value: "auth-1", label: "Claude Primary", attempt_count: 20 }], other_count: 0 },
}

const modelSupportPayload = {
  scope_complete: false,
  selected_count: 2,
  loaded_count: 1,
  accounts: [{
    identity_id: 501,
    auth_index: "codex-auth-e2e",
    display_name: "Codex Auth",
    provider: "Codex",
    channel: "codex",
    disabled: false,
    unavailable: true,
    status: "loaded",
    catalog_status: "loaded",
    registered_models: [{
      id: "gpt-exact",
      display_name: "GPT Exact",
      type: "model",
      definition_status: "available",
      capability: { context_length: 200_000, supported_input_modalities: ["TEXT", "IMAGE"], thinking: { zero_allowed: false } },
    }],
  }, {
    identity_id: 502,
    auth_index: "openai-auth-e2e",
    display_name: "Unsupported OpenAI",
    provider: "OpenAI",
    disabled: true,
    unavailable: true,
    status: "failed",
    error_code: "upstream_error",
    catalog_status: "error",
    registered_models: [],
  }],
  models: [{
    model_id: "gpt-exact",
    display_name: "GPT Exact",
    observed_supporting_accounts: 1,
    selected_accounts: 2,
    single_registered_account_in_scope: null,
  }],
  limits: { max_accounts: 12, max_concurrency: 4, timeout_seconds: 15, max_upstream_requests: 36 },
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
      cache_read_tokens: 100000,
      canonical_valid_attempts: 1,
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
      cache_read_tokens: 10000,
      canonical_valid_attempts: 1,
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
      cache_read_tokens: 50000,
      canonical_valid_attempts: 1,
      request_count: 1,
    },
  ],
}

export const { comparison: _comparison, heatmap, previous_range_start: _previousRangeStart, previous_range_end: _previousRangeEnd, ...dashboardAnalyticsCore } = dashboardAnalyticsSummary

export interface RecordedAPIRequest {
  path: string
  method: string
  url: URL
  body: string | null
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
  await page.route("**/metrics", async (route) => {
    await route.fulfill({ json: metricsPayload })
  })

  await page.route("**/api/v1/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const apiIndex = url.pathname.indexOf("/api/v1")
    const path = apiIndex >= 0 ? url.pathname.slice(apiIndex + "/api/v1".length) : url.pathname
    const method = request.method()
    options.onRequest?.({ path, method, url, body: request.postData() })

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
    if (path === "/usage/failures") {
      await route.fulfill({ json: usageFailureDistribution })
      return
    }
    if (path === "/usage/performance") {
      await route.fulfill({ json: usageAttemptPerformance })
      return
    }
    if (path === "/usage/model-mappings") {
      await route.fulfill({ json: usageModelMappings })
      return
    }
    if (path === "/usage/events/export") {
      await route.fulfill({
        status: 200,
        contentType: "text/csv; charset=utf-8",
        headers: { "Content-Disposition": 'attachment; filename="request-evidence.csv"' },
        body: "timestamp_utc,account,actual_model,request_id\n2026-09-07T11:59:00Z,auth-1,sonnet,request-correlated-e2e\n",
      })
      return
    }
    if (path === "/usage/events") {
      const page = Number(url.searchParams.get("page") ?? "1")
      const pageSize = Number(url.searchParams.get("page_size") ?? "100")
      const provider = url.searchParams.get("provider")?.trim() ?? ""
      if (![1, 10, 20, 50, 100, 500, 1000].includes(pageSize)) {
        await route.fulfill({ status: 400, json: { error: `invalid page_size ${pageSize}` } })
        return
      }
      const start = (page - 1) * pageSize
      const scopedEvents = provider
        ? usageEvents.map((event) => ({
            ...event,
            model: `${provider}-evidence-model`,
            source: provider,
            auth_index: provider,
            api_key_alias: `${provider} Agent`,
          }))
        : usageEvents
      await route.fulfill({ json: {
        events: scopedEvents.slice(start, start + pageSize),
        window_end: url.searchParams.get("window_end") || "2026-09-07T12:00:00.123456789Z",
        total_count: scopedEvents.length,
        page,
        page_size: pageSize,
        total_pages: Math.max(1, Math.ceil(scopedEvents.length / pageSize)),
      } })
      return
    }
    if (path === "/usage/identities/page") {
      const payload = url.searchParams.get("auth_type") === "1" ? authFileIdentitiesPayload : usageIdentities
      const pageSize = Number(url.searchParams.get("page_size") ?? "10")
      await route.fulfill({ json: { ...payload, page_size: pageSize, total_pages: Math.max(1, Math.ceil(payload.total_count / pageSize)) } })
      return
    }
    if (path === "/usage/identities/model-support" && method === "POST") {
      await route.fulfill({ json: modelSupportPayload })
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
