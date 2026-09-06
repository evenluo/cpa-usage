import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageAttemptPerformance } from "@/types/api"

export function buildAttemptPerformancePath(provider: string): string {
  const params = new URLSearchParams({ range: "24h" })
  if (provider) params.set("provider", provider)
  return `/usage/performance?${params.toString()}`
}

export function useAttemptPerformance(provider: string) {
  return useQuery({
    queryKey: ["usage", "performance", "24h", provider],
    queryFn: () => apiFetch<UsageAttemptPerformance>(buildAttemptPerformancePath(provider)),
    staleTime: 30_000,
  })
}
