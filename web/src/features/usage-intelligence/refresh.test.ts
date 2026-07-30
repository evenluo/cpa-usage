import { act, renderHook } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import {
  DASHBOARD_REFRESH_INTERVAL_MS,
  shouldRefreshDashboard,
  useVisibilityRefresh,
} from "./refresh"

describe("dashboard refresh policy", () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it("refreshes only while the tab is visible", () => {
    expect(shouldRefreshDashboard("visible")).toBe(true)
    expect(shouldRefreshDashboard("hidden")).toBe(false)
  })

  it("ticks on the dashboard interval and skips ticks while hidden", () => {
    vi.useFakeTimers()
    const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible")
    const onRefresh = vi.fn()
    const { unmount } = renderHook(() => useVisibilityRefresh(onRefresh))

    act(() => {
      vi.advanceTimersByTime(DASHBOARD_REFRESH_INTERVAL_MS * 2)
    })
    expect(onRefresh).toHaveBeenCalledTimes(2)

    visibility.mockReturnValue("hidden")
    act(() => {
      vi.advanceTimersByTime(DASHBOARD_REFRESH_INTERVAL_MS * 3)
    })
    expect(onRefresh).toHaveBeenCalledTimes(2)

    unmount()
  })

  it("stops ticking after unmount", () => {
    vi.useFakeTimers()
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible")
    const onRefresh = vi.fn()
    const { unmount } = renderHook(() => useVisibilityRefresh(onRefresh, 1_000))

    unmount()
    act(() => {
      vi.advanceTimersByTime(5_000)
    })
    expect(onRefresh).not.toHaveBeenCalled()
  })
})
