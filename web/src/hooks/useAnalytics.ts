import { useQuery } from "@tanstack/react-query"
import { getDefaultGranularity } from "@/features/usage-intelligence/view-model"
import { apiFetch } from "@/lib/api"
import type { AnalyticsCoreResponse, AnalyticsResponse, TimeRange, TimeGranularity } from "@/types/api"

function rangeParam(range: TimeRange): string {
  switch (range) {
    case "today": return "today"
    case "yesterday": return "yesterday"
    case "24h": return "24h"
    case "7d": return "7d"
    case "30d": return "30d"
  }
}

export function buildAnalyticsCorePath(
  range: TimeRange,
  granularity: TimeGranularity,
  provider: string,
): string {
  return buildAnalyticsPath("/analytics/core", range, granularity, provider)
}

export function buildAnalyticsSummaryPath(
  range: TimeRange,
  granularity: TimeGranularity,
  provider: string,
): string {
  return buildAnalyticsPath("/analytics/summary", range, granularity, provider)
}

function buildAnalyticsPath(
  path: string,
  range: TimeRange,
  granularity: TimeGranularity,
  provider: string,
): string {
  const params = new URLSearchParams({
    range: rangeParam(range),
    granularity,
  })
  if (provider) params.set("provider", provider)
  return `${path}?${params.toString()}`
}

export function useAnalytics(
  range: TimeRange,
  granularity: TimeGranularity | null,
  provider: string,
) {
  const g = granularity ?? getDefaultGranularity(range)

  return useQuery({
    queryKey: ["analytics", "summary", range, g, provider],
    queryFn: () =>
      apiFetch<AnalyticsResponse>(buildAnalyticsSummaryPath(range, g, provider)),
    staleTime: 30_000,
    refetchInterval: () => {
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return false
      }
      return 60_000
    },
  })
}

export function useAnalyticsCore(
  range: TimeRange,
  granularity: TimeGranularity | null,
  provider: string,
) {
  const g = granularity ?? getDefaultGranularity(range)

  return useQuery({
    queryKey: ["analytics", "core", range, g, provider],
    queryFn: () =>
      apiFetch<AnalyticsCoreResponse>(buildAnalyticsCorePath(range, g, provider)),
    staleTime: 30_000,
    refetchInterval: () => {
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return false
      }
      return 60_000
    },
  })
}

export function mergeAnalyticsCore(
  full: AnalyticsResponse | undefined,
  core: AnalyticsCoreResponse | undefined,
): AnalyticsResponse | undefined {
  if (!full && !core) return undefined
  if (!full) return core
  if (!core) return full
  return {
    ...full,
    ...core,
    granularity: core.granularity ?? full.granularity,
    summary: core.summary,
    trend: core.trend,
    time_breakdown: core.trend,
  }
}
