import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageOverviewResponse } from "@/types/api"

export function useUsageOverview(range: string, provider: string) {
  const params = new URLSearchParams({ range })
  if (provider) params.set("provider", provider)

  return useQuery({
    queryKey: ["usage", "overview", range, provider],
    queryFn: () =>
      apiFetch<UsageOverviewResponse>(`/usage/overview?${params.toString()}`),
    staleTime: 30_000,
    refetchInterval: (_query) => {
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return false
      }
      return 60_000
    },
  })
}
