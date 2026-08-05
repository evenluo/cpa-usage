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
  useAnalyticsHeatmap: vi.fn(() => ({ data: undefined, isLoading: false, error: undefined })),
}))
vi.mock("@/hooks/useEvents", () => ({
  useEvents: vi.fn(() => ({ data: undefined, isLoading: false, isFetching: false, refetch: vi.fn(), error: undefined })),
}))
vi.mock("@/hooks/useRequestHealth", () => ({
  useRequestHealth: vi.fn(() => ({ data: undefined, isLoading: false, error: undefined })),
}))
vi.mock("./refresh", () => ({
  useVisibilityRefresh: vi.fn(),
}))

import { useAnalyticsCore } from "@/hooks/useAnalytics"
import { useEvents } from "@/hooks/useEvents"

describe("stored time range helpers", () => {
  beforeEach(() => {
    window.localStorage.clear()
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
  })

  it("refreshDashboard refetches the core analytics and request evidence queries", async () => {
    const refetchCore = vi.fn().mockResolvedValue(undefined)
    const refetchEvidence = vi.fn().mockResolvedValue(undefined)
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

    const { result } = renderHook(() => useUsageDashboard())
    act(() => {
      result.current.refreshDashboard()
    })

    await vi.waitFor(() => {
      expect(refetchCore).toHaveBeenCalledTimes(1)
      expect(refetchEvidence).toHaveBeenCalledTimes(1)
    })
  })
})
