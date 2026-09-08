import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageAttemptPerformance } from "@/types/api"

export function buildAttemptPerformancePath(provider: string): string {
  const params = new URLSearchParams({ range: "24h" })
  if (provider) params.set("provider", provider)
  return `/usage/performance?${params.toString()}`
}

export function useAttemptPerformance(provider: string, enabled = true) {
  return useQuery({
    queryKey: ["usage", "performance", "24h", provider],
    queryFn: ({ signal }) => apiFetch<UsageAttemptPerformance>(buildAttemptPerformancePath(provider), { signal }),
    enabled,
    staleTime: 60_000,
  })
}
