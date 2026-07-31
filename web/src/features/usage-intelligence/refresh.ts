import { useEffect } from "react"

export const DASHBOARD_REFRESH_INTERVAL_MS = 60_000

/** Refresh ticks are skipped while the tab is hidden. */
export function shouldRefreshDashboard(visibilityState: Document["visibilityState"]): boolean {
  return visibilityState !== "hidden"
}

export function useVisibilityRefresh(
  onRefresh: () => void,
  intervalMs: number = DASHBOARD_REFRESH_INTERVAL_MS,
) {
  useEffect(() => {
    const intervalID = window.setInterval(() => {
      if (!shouldRefreshDashboard(document.visibilityState)) return
      onRefresh()
    }, intervalMs)

    return () => window.clearInterval(intervalID)
  }, [onRefresh, intervalMs])
}
