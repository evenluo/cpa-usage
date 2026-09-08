import { act, renderHook } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import {
  readStoredTimeRange,
  useUsageDashboard,
  writeStoredTimeRange,
} from "./use-usage-dashboard"
import { DEFAULT_TIME_RANGE, SELECTED_TIME_RANGE_STORAGE_KEY } from "./view-model"

vi.mock("@/hooks/useAnalytics", () => ({
  useAnalyticsCore: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
  useAnalyticsHeatmap: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("@/hooks/useEvents", () => ({
  useEvents: vi.fn(() => ({ data: undefined, isLoading: false, isFetching: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("@/hooks/useRequestHealth", () => ({
  useRequestHealth: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("@/hooks/useFailureDistribution", () => ({
  useFailureDistribution: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("@/hooks/useModelMappings", () => ({
  useModelMappings: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
  useModelMappingsSummary: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("@/hooks/useAttemptPerformance", () => ({
  useAttemptPerformance: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("@/hooks/usePerformanceProviders", () => ({
  usePerformanceProviders: vi.fn(() => ({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("./refresh", () => ({
  useVisibilityRefresh: vi.fn(),
}))

import { useAnalyticsCore, useAnalyticsHeatmap } from "@/hooks/useAnalytics"
import { useEvents } from "@/hooks/useEvents"
import { useRequestHealth } from "@/hooks/useRequestHealth"
import { useFailureDistribution } from "@/hooks/useFailureDistribution"
import { useModelMappings, useModelMappingsSummary } from "@/hooks/useModelMappings"
import { useAttemptPerformance } from "@/hooks/useAttemptPerformance"
import { usePerformanceProviders } from "@/hooks/usePerformanceProviders"

describe("stored time range helpers", () => {
  beforeEach(() => {
    window.localStorage.clear()
    vi.mocked(useAnalyticsCore).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(useAnalyticsHeatmap).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(useEvents).mockReturnValue({ data: undefined, isLoading: false, isFetching: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(useRequestHealth).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(useFailureDistribution).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(useModelMappings).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(useModelMappingsSummary).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(useAttemptPerformance).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
    vi.mocked(usePerformanceProviders).mockReturnValue({ data: undefined, isLoading: false, refetch: vi.fn(), error: undefined } as never)
  })

  it("defaults to the dashboard default when nothing is stored", () => {
    expect(readStoredTimeRange()).toBe(DEFAULT_TIME_RANGE)
  })

  it("reads a stored valid range and ignores invalid values", () => {
    window.localStorage.setItem(SELECTED_TIME_RANGE_STORAGE_KEY, "30d")
    expect(readStoredTimeRange()).toBe("30d")

    window.localStorage.setItem(SELECTED_TIME_RANGE_STORAGE_KEY, "bogus")
    expect(readStoredTimeRange()).toBe(DEFAULT_TIME_RANGE)
  })

  it("falls back to the default when storage access throws", () => {
    const getItem = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("storage blocked")
    })
    expect(readStoredTimeRange()).toBe(DEFAULT_TIME_RANGE)
    getItem.mockRestore()
  })

  it("writeStoredTimeRange persists the range and swallows storage errors", () => {
    writeStoredTimeRange("24h")
    expect(window.localStorage.getItem(SELECTED_TIME_RANGE_STORAGE_KEY)).toBe("24h")

    const setItem = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("storage blocked")
    })
    expect(() => writeStoredTimeRange("7d")).not.toThrow()
    setItem.mockRestore()
  })
})

describe("useUsageDashboard", () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it("starts with the stored range and the range default granularity", () => {
    window.localStorage.setItem(SELECTED_TIME_RANGE_STORAGE_KEY, "30d")
    const { result } = renderHook(() => useUsageDashboard())

    expect(result.current.range).toBe("30d")
    expect(result.current.granularity).toBeNull()
    expect(result.current.effectiveGranularity).toBe("day")
  })

  it("selectRange switches the range, resets granularity, and persists the selection", () => {
    const { result } = renderHook(() => useUsageDashboard())

    act(() => {
      result.current.setGranularity("day")
    })
    expect(result.current.effectiveGranularity).toBe("day")

    act(() => {
      result.current.selectRange("24h")
    })

    expect(result.current.range).toBe("24h")
    expect(result.current.granularity).toBeNull()
    expect(result.current.effectiveGranularity).toBe("hour")
    expect(window.localStorage.getItem(SELECTED_TIME_RANGE_STORAGE_KEY)).toBe("24h")
  })

  it("derives the selected analytics query from range, granularity, and provider", () => {
    const { result } = renderHook(() => useUsageDashboard())

    act(() => {
      result.current.selectRange("7d")
      result.current.setProvider("claude")
    })

    expect(result.current.loadPlan.selectedWindow.analytics).toEqual({
      range: "7d",
      granularity: "hour",
      provider: "claude",
    })
    expect(useAnalyticsCore).toHaveBeenCalledWith("7d", "hour", "claude", false)
  })

  it("keeps the fixed operational window queries independent of the selected range", () => {
    renderHook(() => useUsageDashboard())

    const analyticsCalls = vi.mocked(useAnalyticsCore).mock.calls
    const eventsCalls = vi.mocked(useEvents).mock.calls
    expect(analyticsCalls[0].slice(0, 3)).toEqual(["7d", "hour", ""])
    expect(eventsCalls[0].slice(0, 4)).toEqual(["24h", 1, "", 1])
    expect(useFailureDistribution).toHaveBeenCalledWith("")
    expect(useModelMappings).toHaveBeenCalledWith("", false, "")
    expect(useModelMappingsSummary).toHaveBeenCalledWith("")
    expect(useAttemptPerformance).toHaveBeenCalledWith("", false)
  })

  it("refreshDashboard refetches the core analytics and fixed diagnostic queries", async () => {
    const refetchCore = vi.fn().mockResolvedValue(undefined)
    const refetchEvidence = vi.fn().mockResolvedValue(undefined)
    const refetchFailures = vi.fn().mockResolvedValue(undefined)
    const refetchMappings = vi.fn().mockResolvedValue(undefined)
    const refetchPerformance = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useAnalyticsCore).mockReturnValue({
      data: undefined,
      isLoading: false,
      refetch: refetchCore,
      error: undefined,
    } as never)
    vi.mocked(useEvents).mockReturnValue({
      data: undefined,
      isLoading: false,
      isFetching: false,
      refetch: refetchEvidence,
      error: undefined,
    } as never)
    vi.mocked(useFailureDistribution).mockReturnValue({ data: undefined, isLoading: false, refetch: refetchFailures, error: undefined } as never)
    vi.mocked(useModelMappingsSummary).mockReturnValue({ data: undefined, isLoading: false, refetch: refetchMappings, error: undefined } as never)
    vi.mocked(useAttemptPerformance).mockReturnValue({ data: undefined, isLoading: false, refetch: refetchPerformance, error: undefined } as never)

    const { result } = renderHook(() => useUsageDashboard())
    act(() => {
      result.current.refreshDashboard()
    })

    await vi.waitFor(() => {
      expect(refetchCore).toHaveBeenCalledTimes(1)
      expect(refetchEvidence).toHaveBeenCalledTimes(1)
      expect(refetchFailures).toHaveBeenCalledTimes(1)
      expect(refetchMappings).toHaveBeenCalledTimes(1)
      expect(refetchPerformance).not.toHaveBeenCalled()
    })
  })

  it("exposes independent retry commands for every dashboard read", () => {
    const retryCore = vi.fn()
    const retryHeatmap = vi.fn()
    const retryEvidence = vi.fn()
    const retryHealth = vi.fn()
    const retryFailures = vi.fn()
    const retryMappings = vi.fn()
    const retryPerformance = vi.fn()
    vi.mocked(useAnalyticsCore).mockReturnValue({ data: undefined, isLoading: false, refetch: retryCore, error: new Error("core") } as never)
    vi.mocked(useAnalyticsHeatmap).mockReturnValue({ data: undefined, isLoading: false, refetch: retryHeatmap, error: new Error("heatmap") } as never)
    vi.mocked(useEvents).mockReturnValue({ data: undefined, isLoading: false, isFetching: false, refetch: retryEvidence, error: new Error("evidence") } as never)
    vi.mocked(useRequestHealth).mockReturnValue({ data: undefined, isLoading: false, refetch: retryHealth, error: new Error("health") } as never)
    vi.mocked(useFailureDistribution).mockReturnValue({ data: undefined, isLoading: false, refetch: retryFailures, error: new Error("failures") } as never)
    vi.mocked(useModelMappingsSummary).mockReturnValue({ data: undefined, isLoading: false, refetch: retryMappings, error: new Error("mappings") } as never)
    vi.mocked(useAttemptPerformance).mockReturnValue({ data: undefined, isLoading: false, refetch: retryPerformance, error: new Error("performance") } as never)

    const { result } = renderHook(() => useUsageDashboard())
    act(() => {
      result.current.retryCore()
      result.current.retryHeatmap()
      result.current.retryRequestEvidence()
      result.current.retryRequestHealth()
      result.current.retryFailureDistribution()
      result.current.retryModelMappingsSummary()
      result.current.retryAttemptPerformance()
    })

    expect(retryCore).toHaveBeenCalledTimes(1)
    expect(retryHeatmap).toHaveBeenCalledTimes(1)
    expect(retryEvidence).toHaveBeenCalledTimes(1)
    expect(retryHealth).toHaveBeenCalledTimes(1)
    expect(retryFailures).toHaveBeenCalledTimes(1)
    expect(retryMappings).toHaveBeenCalledTimes(1)
    expect(retryPerformance).not.toHaveBeenCalled()
  })

  it("loads model-mapping rows only after expansion against the summary snapshot", () => {
    const windowEnd = "2026-09-08T12:00:00Z"
    let summary = {
      window_start: "2026-09-07T12:00:00Z",
      window_end: windowEnd,
      total_attempts: 3,
      observed_alias_attempts: 2,
      missing_alias_attempts: 1,
      alias_coverage: 200 / 3,
      displayed_mappings: 2,
    }
    vi.mocked(useModelMappingsSummary).mockImplementation(() => ({
      data: summary,
      isLoading: false,
      refetch: vi.fn(),
      error: null,
    }) as never)

    const { result, rerender } = renderHook(() => useUsageDashboard())
    expect(useModelMappings).toHaveBeenLastCalledWith("", false, "")

    act(() => result.current.setModelMappingsExpanded(true))

    expect(useModelMappings).toHaveBeenLastCalledWith("", true, windowEnd)
    summary = { ...summary, window_start: "2026-09-07T12:01:00Z", window_end: "2026-09-08T12:01:00Z" }
    rerender()
    expect(result.current.modelMappingsSummaryData?.window_end).toBe(windowEnd)
    expect(useModelMappings).toHaveBeenLastCalledWith("", true, windowEnd)

    act(() => result.current.setModelMappingsExpanded(false))
    expect(result.current.modelMappingsSummaryData?.window_end).toBe("2026-09-08T12:01:00Z")
    expect(useModelMappings).toHaveBeenLastCalledWith("", false, windowEnd)
  })
})


describe("performance provider selection", () => {
  afterEach(() => vi.clearAllMocks())

  it("defaults from the complete 24h catalog once and isolates manual selection", () => {
    const options = Array.from({ length: 9 }, (_, index) => ({ provider: `provider-${index}`, request_count: index + 1 }))
    let catalog: object | undefined
    vi.mocked(usePerformanceProviders).mockImplementation(() => ({ data: catalog, isLoading: !catalog, error: null, refetch: vi.fn() }) as never)
    vi.mocked(useAttemptPerformance).mockReturnValue({ data: undefined, isLoading: false, error: null, refetch: vi.fn() } as never)
    const { result, rerender } = renderHook(() => useUsageDashboard())
    expect(useAttemptPerformance).toHaveBeenLastCalledWith("", false)
    catalog = { window_start: "2026-09-07T00:00:00Z", window_end: "2026-09-08T00:00:00Z", provider_options: options }
    rerender()
    expect(result.current.attemptPerformanceProvider).toBe("provider-8")
    expect(result.current.attemptPerformanceProviders).toHaveLength(9)
    expect(useAttemptPerformance).toHaveBeenLastCalledWith("provider-8", true)
    catalog = { window_start: "2026-09-07T00:00:00Z", window_end: "2026-09-08T00:00:00Z", provider_options: [{ provider: "changed-leader", request_count: 100 }] }
    rerender()
    expect(result.current.attemptPerformanceProvider).toBe("provider-8")
    expect(result.current.attemptPerformanceProviders).toContain("provider-8")
    act(() => {
      result.current.setAttemptPerformanceProvider("changed-leader")
      result.current.setProvider("global-provider")
      result.current.selectRange("30d")
    })
    expect(result.current.attemptPerformanceProvider).toBe("changed-leader")
    expect(useAttemptPerformance).toHaveBeenLastCalledWith("changed-leader", true)
    expect(result.current.loadPlan.fixedWindow.attemptPerformance.provider).toBe("changed-leader")
    expect(result.current.loadPlan.fixedWindow.requestHealth.provider).toBe("global-provider")
  })

  it("separates provider-list refresh errors from scoped result errors and retry owners", () => {
    const retryCatalog = vi.fn()
    const retryScoped = vi.fn()
    const catalogError = new Error("catalog refresh failed")
    vi.mocked(usePerformanceProviders).mockReturnValue({ data: { window_start: "", window_end: "", provider_options: [{ provider: "a", request_count: 1 }] }, isLoading: false, error: catalogError, refetch: retryCatalog } as never)
    vi.mocked(useAttemptPerformance).mockReturnValue({ data: undefined, isLoading: true, error: null, refetch: retryScoped } as never)
    const { result, rerender } = renderHook(() => useUsageDashboard())
    expect(result.current.performanceProvidersError).toBe(catalogError)
    expect(result.current.attemptPerformanceError).toBeNull()
    expect(result.current.isAttemptPerformanceLoading).toBe(true)
    act(() => result.current.retryAttemptPerformance())
    expect(retryScoped).toHaveBeenCalledTimes(1)
    expect(retryCatalog).not.toHaveBeenCalled()
    act(() => result.current.retryPerformanceProviders())
    expect(retryCatalog).toHaveBeenCalledTimes(1)
    const scopedError = new Error("selected provider failed")
    vi.mocked(useAttemptPerformance).mockReturnValue({ data: undefined, isLoading: false, error: scopedError, refetch: retryScoped } as never)
    rerender()
    expect(result.current.attemptPerformanceError).toBe(scopedError)
  })

  it("breaks default request-count ties by provider and exposes discovery failures for retry", () => {
    const retry = vi.fn()
    vi.mocked(usePerformanceProviders).mockReturnValue({ data: undefined, isLoading: false, error: new Error("catalog unavailable"), refetch: retry } as never)
    const { result, rerender } = renderHook(() => useUsageDashboard())
    expect(result.current.attemptPerformanceProvider).toBe("")
    expect(result.current.attemptPerformanceError).toBeTruthy()
    act(() => result.current.retryAttemptPerformance())
    expect(retry).toHaveBeenCalledTimes(1)
    vi.mocked(usePerformanceProviders).mockReturnValue({ data: { window_start: "", window_end: "", provider_options: [{ provider: "z", request_count: 10 }, { provider: "a", request_count: 10 }] }, isLoading: false, error: null, refetch: retry } as never)
    rerender()
    expect(result.current.attemptPerformanceProvider).toBe("a")
  })
})
