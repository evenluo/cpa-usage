import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"

export interface UsagePerformanceProviderOption {
  provider: string
  request_count: number
}

export interface UsagePerformanceProvidersResponse {
  window_start: string
  window_end: string
  provider_options: UsagePerformanceProviderOption[]
}

export function buildPerformanceProvidersPath(): string {
  return "/usage/performance/providers?range=24h"
}

export function usePerformanceProviders() {
  return useQuery({
    queryKey: ["usage", "performance", "providers", "24h"],
    queryFn: ({ signal }) => apiFetch<UsagePerformanceProvidersResponse>(buildPerformanceProvidersPath(), { signal }),
    staleTime: 60_000,
  })
}
