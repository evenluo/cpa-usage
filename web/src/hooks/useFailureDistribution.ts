import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageFailureDistribution } from "@/types/api"

export function buildFailureDistributionPath(provider: string): string {
  const params = new URLSearchParams({ range: "24h" })
  if (provider) params.set("provider", provider)
  return `/usage/failures?${params.toString()}`
}

export function useFailureDistribution(provider: string) {
  return useQuery({
    queryKey: ["usage", "failures", "24h", provider],
    queryFn: ({ signal }) => apiFetch<UsageFailureDistribution>(buildFailureDistributionPath(provider), { signal }),
    staleTime: 60_000,
  })
}
