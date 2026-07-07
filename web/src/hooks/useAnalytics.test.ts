import { describe, expect, it } from "vitest"
import type { AnalyticsCoreResponse, AnalyticsResponse, AnalyticsSummary, TrendPoint } from "@/types/api"
import { buildAnalyticsCorePath, buildAnalyticsSummaryPath, mergeAnalyticsCore } from "./useAnalytics"

function summary(totalTokens: number): AnalyticsSummary {
  return {
    total_cost: totalTokens,
    total_tokens: totalTokens,
    request_count: totalTokens,
    success_count: totalTokens,
    failure_count: 0,
    input_tokens: totalTokens,
    output_tokens: 0,
    reasoning_tokens: 0,
    cached_tokens: 0,
    success_rate: 100,
    cost_available: true,
    cost_status: "available",
    cache_read_share: 0,
    cache_read_share_state: "no_cache_data",
  }
}

function trendPoint(label: string, totalTokens: number): TrendPoint {
  return {
    label,
    total_cost: totalTokens,
    total_tokens: totalTokens,
    input_tokens: totalTokens,
    output_tokens: 0,
    reasoning_tokens: 0,
    cached_tokens: 0,
    request_count: totalTokens,
    success_count: totalTokens,
    failure_count: 0,
    cost_available: true,
    cost_status: "available",
  }
}

describe("useAnalytics", () => {
  it("reads Usage Intelligence core analytics from the core endpoint", () => {
    expect(buildAnalyticsCorePath("7d", "hour", "OpenAI")).toBe(
      "/analytics/core?range=7d&granularity=hour&provider=OpenAI",
    )
  })

  it("keeps the full analytics hook on the summary endpoint", () => {
    expect(buildAnalyticsSummaryPath("7d", "hour", "OpenAI")).toBe(
      "/analytics/summary?range=7d&granularity=hour&provider=OpenAI",
    )
  })

  it("omits provider when all providers are selected", () => {
    expect(buildAnalyticsCorePath("30d", "day", "")).toBe(
      "/analytics/core?range=30d&granularity=day",
    )
  })

  it("merges core KPI and trend data without dropping full-dashboard fields", () => {
    const full: AnalyticsResponse = {
      summary: summary(10),
      trend: [trendPoint("full", 10)],
      heatmap: { measure: "tokens", max_tokens: 1, max_cost: 1, max_requests: 1, max_failures: 0, rows: [] },
      provider_options: [{ provider: "OpenAI", request_count: 1, total_tokens: 10, total_cost: 1, cost_available: true, cost_status: "available" }],
      api_key_breakdown: [],
      key_alias_breakdown: [],
    }
    const core: AnalyticsCoreResponse = {
      summary: summary(20),
      trend: [trendPoint("core", 20)],
    }

    expect(mergeAnalyticsCore(full, core)).toEqual({
      ...full,
      summary: core.summary,
      trend: core.trend,
      time_breakdown: core.trend,
    })
  })
})
