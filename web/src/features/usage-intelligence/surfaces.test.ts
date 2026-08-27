import { describe, expect, it } from "vitest"
import type { AnalyticsCoreResponse } from "@/types/api"
import type { UsageDashboardViewModel } from "./view-model"
import {
  buildSurface,
  buildUsageDashboardSurfaces,
  type UsageSurfaceStatus,
} from "./surfaces"

function viewModel(overrides: Partial<UsageDashboardViewModel> = {}): UsageDashboardViewModel {
  return {
    trend: [],
    keyAliases: [],
    apiKeys: [],
    leaderboardRows: [],
    providerOptions: [],
    hasLeaderboardBreakdown: false,
    leaderboardSortLabel: "Sort: Cost",
    kpiData: null,
    ...overrides,
  }
}

const analytics = {
  summary: { request_count: 1 },
  trend: [],
  api_key_breakdown: [],
} as unknown as AnalyticsCoreResponse

describe("buildSurface status matrix", () => {
  const cases: Array<{
    name: string
    hasData: boolean
    isLoading: boolean
    error: unknown
    expected: UsageSurfaceStatus
  }> = [
    { name: "no data + loading → loading", hasData: false, isLoading: true, error: null, expected: "loading" },
    { name: "no data + loading + stale error → loading wins", hasData: false, isLoading: true, error: new Error("boom"), expected: "loading" },
    { name: "no data + error → error", hasData: false, isLoading: false, error: new Error("boom"), expected: "error" },
    { name: "no data + settled → empty", hasData: false, isLoading: false, error: null, expected: "empty" },
    { name: "data + idle → ready", hasData: true, isLoading: false, error: null, expected: "ready" },
    { name: "data + refreshing → ready", hasData: true, isLoading: true, error: null, expected: "ready" },
    { name: "data + refresh error → ready", hasData: true, isLoading: false, error: new Error("boom"), expected: "ready" },
  ]

  it.each(cases)("$name", ({ hasData, isLoading, error, expected }) => {
    const surface = buildSurface({ data: hasData ? { value: 1 } : undefined, isLoading, error })

    expect(surface.status).toBe(expected)
    expect(surface.status === "ready" ? surface.data : undefined).toEqual(
      hasData && expected === "ready" ? { value: 1 } : undefined,
    )
  })
})

describe("buildUsageDashboardSurfaces", () => {
  const idle = { isLoading: false, error: null }

  it("marks Selected Analysis Window surfaces loading while the core query has no data", () => {
    const surfaces = buildUsageDashboardSurfaces({
      viewModel: viewModel(),
      core: { data: undefined, isLoading: true, error: null },
      heatmap: idle,
      requestHealth: idle,
    })

    expect(surfaces.kpis.status).toBe("loading")
    expect(surfaces.trend.status).toBe("loading")
    expect(surfaces.leaderboard.status).toBe("loading")
  })

  it("marks every core-owned surface error without cached data", () => {
    const surfaces = buildUsageDashboardSurfaces({
      viewModel: viewModel(),
      core: { data: undefined, isLoading: false, error: new Error("boom") },
      heatmap: idle,
      requestHealth: idle,
    })

    expect(surfaces.kpis.status).toBe("error")
    expect(surfaces.trend.status).toBe("error")
    expect(surfaces.core.status).toBe("error")
    expect(surfaces.leaderboard.status).toBe("error")
    expect(surfaces.leaderboard.data).toEqual([])
  })

  it("keeps surfaces ready with cached data while a refresh is in flight or fails", () => {
    const rows = viewModel({
      hasLeaderboardBreakdown: true,
      leaderboardRows: [{ label: "key" } as UsageDashboardViewModel["leaderboardRows"][number]],
    })

    for (const core of [
      { data: analytics, isLoading: true, error: null },
      { data: analytics, isLoading: false, error: new Error("boom") },
    ]) {
      const surfaces = buildUsageDashboardSurfaces({
        viewModel: rows,
        core,
        heatmap: idle,
        requestHealth: idle,
      })

      expect(surfaces.kpis.status).toBe("ready")
      expect(surfaces.trend.status).toBe("ready")
      expect(surfaces.leaderboard.status).toBe("ready")
      expect(surfaces.leaderboard.data).toEqual(rows.leaderboardRows)
    }
  })

  it("renders leaderboard rows even when the breakdown is absent and the core query settled", () => {
    const surfaces = buildUsageDashboardSurfaces({
      viewModel: viewModel({ hasLeaderboardBreakdown: false }),
      core: { data: analytics, isLoading: false, error: null },
      heatmap: idle,
      requestHealth: idle,
    })

    expect(surfaces.leaderboard.status).toBe("empty")
  })

  it("derives Fixed Operational Window surfaces from view-model data presence", () => {
    const vm = viewModel({
      fixedHeatmap: {
        measure: "tokens",
        max_tokens: 1,
        max_cost: 1,
        max_requests: 1,
        max_failures: 0,
        rows: [{
          date: "2026-05-18",
          label: "May 18",
          cells: [{
            hour: 0,
            in_range: true,
            bucket_start: "2026-05-18T00:00:00Z",
            bucket_end: "2026-05-18T01:00:00Z",
            total_tokens: 1,
            total_cost: 0,
            request_count: 1,
            failure_count: 0,
            cost_available: true,
            cost_status: "available",
          }],
        }],
      },
      serviceHealth: {
        total_success: 1,
        total_failure: 0,
        success_rate: 100,
        rows: 1,
        columns: 1,
        bucket_seconds: 180,
        window_start: "2026-05-18T00:00:00Z",
        window_end: "2026-05-19T00:00:00Z",
        block_details: [],
      },
    })

    const loading = buildUsageDashboardSurfaces({
      viewModel: viewModel(),
      core: { data: analytics, isLoading: false, error: null },
      heatmap: { isLoading: true, error: null },
      requestHealth: { isLoading: true, error: null },
    })
    expect(loading.heatmap.status).toBe("loading")
    expect(loading.requestHealth.status).toBe("loading")

    const failed = buildUsageDashboardSurfaces({
      viewModel: viewModel(),
      core: { data: analytics, isLoading: false, error: null },
      heatmap: { isLoading: false, error: new Error("boom") },
      requestHealth: { isLoading: false, error: new Error("boom") },
    })
    expect(failed.heatmap.status).toBe("error")
    expect(failed.requestHealth.status).toBe("error")

    const ready = buildUsageDashboardSurfaces({
      viewModel: vm,
      core: { data: analytics, isLoading: false, error: null },
      heatmap: { isLoading: true, error: new Error("boom") },
      requestHealth: idle,
    })
    expect(ready.heatmap).toMatchObject({ status: "ready", data: vm.fixedHeatmap, refreshError: expect.any(Error) })
    expect(ready.requestHealth).toEqual({ status: "ready", data: vm.serviceHealth })
  })

  it("reports empty fixed surfaces when queries settle without data", () => {
    const surfaces = buildUsageDashboardSurfaces({
      viewModel: viewModel(),
      core: { data: analytics, isLoading: false, error: null },
      heatmap: idle,
      requestHealth: idle,
    })

    expect(surfaces.heatmap.status).toBe("empty")
    expect(surfaces.requestHealth.status).toBe("empty")
  })

  it("reports successful zero-valued payloads as empty and keeps stale refresh errors observable", () => {
    const staleError = new Error("refresh failed")
    const surfaces = buildUsageDashboardSurfaces({
      viewModel: viewModel({
        fixedHeatmap: { measure: "tokens", max_tokens: 0, max_cost: 0, max_requests: 0, max_failures: 0, rows: [] },
        serviceHealth: {
          total_success: 0,
          total_failure: 0,
          success_rate: 0,
          rows: 0,
          columns: 0,
          bucket_seconds: 180,
          window_start: "2026-05-18T00:00:00Z",
          window_end: "2026-05-19T00:00:00Z",
          block_details: [],
        },
      }),
      core: {
        data: { ...analytics, summary: { request_count: 0 } } as AnalyticsCoreResponse,
        isLoading: false,
        error: staleError,
      },
      heatmap: { isLoading: false, error: staleError },
      requestHealth: { isLoading: false, error: staleError },
    })

    expect(surfaces.core).toMatchObject({ status: "empty", refreshError: staleError })
    expect(surfaces.heatmap).toMatchObject({ status: "empty", refreshError: staleError })
    expect(surfaces.requestHealth).toMatchObject({ status: "empty", refreshError: staleError })
  })
})
