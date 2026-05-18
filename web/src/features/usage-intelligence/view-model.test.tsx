import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it } from "vitest"
import type { AnalyticsResponse, KeyAliasBreakdown, TrendPoint } from "@/types/api"
import {
  buildUsageDashboardViewModel,
  deriveKpiSparklineData,
  getDefaultGranularity,
  getEffectiveGranularity,
  getLeaderboardSortLabel,
  TIME_RANGES,
} from "./view-model"

function trendPoint(overrides: Partial<TrendPoint>): TrendPoint {
  return {
    label: "bucket",
    total_cost: 0,
    total_tokens: 0,
    request_count: 0,
    success_count: 0,
    failure_count: 0,
    cost_available: true,
    cost_status: "available",
    ...overrides,
  }
}

function keyRow(label: string): KeyAliasBreakdown {
  return {
    label,
    alias: label,
    traceability: `${label}-trace`,
    identity: `${label}-identity`,
    auth_type: 1,
    auth_type_name: "apikey",
    type: "openai",
    provider: "OpenAI",
    is_deleted: false,
    total_cost: 0,
    total_tokens: 0,
    request_count: 0,
    success_count: 0,
    failure_count: 0,
    success_rate: 100,
    last_used_at: null,
    cost_available: true,
    cost_status: "available",
    trend: [],
  }
}

describe("Usage Intelligence view model", () => {
  it("derives default Time Granularity for each Selected Analysis Window", () => {
    expect(TIME_RANGES.map((range) => range.value)).toEqual(["today", "yesterday", "24h", "7d", "30d"])
    expect(getDefaultGranularity("today")).toBe("hour")
    expect(getDefaultGranularity("yesterday")).toBe("hour")
    expect(getDefaultGranularity("24h")).toBe("hour")
    expect(getDefaultGranularity("7d")).toBe("hour")
    expect(getDefaultGranularity("30d")).toBe("day")
    expect(getEffectiveGranularity("30d", "hour")).toBe("hour")
  })

  it("derives KPI sparklines without counting unavailable Cost or negative successes", () => {
    const data = deriveKpiSparklineData([
      trendPoint({ total_cost: 2.5, total_tokens: 100, request_count: 4, failure_count: 1 }),
      trendPoint({ total_cost: 5, total_tokens: 50, request_count: 2, failure_count: 3, cost_status: "unavailable" }),
    ])

    expect(data).toEqual({
      cost: [2.5, null],
      tokens: [100, 50],
      requests: [4, 2],
      successRate: [75, 0],
    })
  })

  it("selects Selected Analysis Window outputs and Fixed Operational Window outputs", () => {
    const apiKey = keyRow("API Key")
    const account = keyRow("Account")
    const analytics: AnalyticsResponse = {
      summary: {
        total_cost: 10,
        total_tokens: 20,
        request_count: 2,
        success_count: 2,
        failure_count: 0,
        input_tokens: 20,
        cached_tokens: 5,
        success_rate: 100,
        cost_available: false,
        cost_status: "partial",
        cache_read_share: 25,
        cache_read_share_state: "available",
      },
      trend: [trendPoint({ label: "selected-window", request_count: 1 })],
      key_alias_breakdown: [account],
      api_key_breakdown: [apiKey],
      provider_options: [{ provider: "OpenAI", request_count: 2, total_tokens: 20, total_cost: 10, cost_available: false, cost_status: "partial" }],
    }
    analytics.heatmap = { measure: "tokens" as const, max_tokens: 20, max_cost: 10, max_requests: 2, max_failures: 0, rows: [] }
    const healthOverview = {
      service_health: {
        total_success: 2,
        total_failure: 0,
        success_rate: 100,
        rows: 1,
        columns: 1,
        bucket_seconds: 180,
        window_start: "2026-05-18T00:00:00Z",
        window_end: "2026-05-19T00:00:00Z",
        block_details: [],
      },
    }

    const viewModel = buildUsageDashboardViewModel({
      analytics,
      healthOverview,
      leaderboardScope: "api-key",
    })

    expect(viewModel.trend[0].label).toBe("selected-window")
    expect(viewModel.leaderboardRows).toEqual([apiKey])
    expect(viewModel.keyAliases).toEqual([account])
    expect(viewModel.providerOptions).toEqual(analytics.provider_options)
    expect(viewModel.fixedHeatmap).toBe(analytics.heatmap)
    expect(viewModel.serviceHealth).toBe(healthOverview.service_health)
    expect(viewModel.leaderboardSortLabel).toBe("Sort: Cost partial")
  })

  it("keeps empty-data behavior explicit", () => {
    expect(buildUsageDashboardViewModel({ leaderboardScope: "account" })).toMatchObject({
      trend: [],
      keyAliases: [],
      apiKeys: [],
      leaderboardRows: [],
      providerOptions: [],
      leaderboardSortLabel: "Sort: Cost",
      kpiData: null,
    })
    expect(getLeaderboardSortLabel("unavailable")).toBe("Sort: Tokens")
  })

  it("runs with Testing Library and user-event", async () => {
    const user = userEvent.setup()
    render(<button type="button">Provider options</button>)

    await user.click(screen.getByRole("button", { name: "Provider options" }))

    expect(screen.getByRole("button", { name: "Provider options" })).toBeInTheDocument()
  })
})
