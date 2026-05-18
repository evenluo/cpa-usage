import { useQuery } from "@tanstack/react-query"
import { getDefaultGranularity } from "@/features/usage-intelligence/view-model"
import { apiFetch } from "@/lib/api"
import type { AnalyticsResponse, TimeRange, TimeGranularity } from "@/types/api"

function rangeParam(range: TimeRange): string {
  switch (range) {
    case "today": return "today"
    case "yesterday": return "yesterday"
    case "24h": return "24h"
    case "7d": return "7d"
    case "30d": return "30d"
  }
}

export function useAnalytics(
  range: TimeRange,
  granularity: TimeGranularity | null,
  provider: string,
) {
  const g = granularity ?? getDefaultGranularity(range)
  const params = new URLSearchParams({
    range: rangeParam(range),
    granularity: g,
  })
  if (provider) params.set("provider", provider)

  return useQuery({
    queryKey: ["analytics", "summary", range, g, provider],
    queryFn: () =>
      apiFetch<AnalyticsResponse>(`/analytics/summary?${params.toString()}`),
    staleTime: 30_000,
    refetchInterval: () => {
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return false
      }
      return 60_000
    },
  })
}
