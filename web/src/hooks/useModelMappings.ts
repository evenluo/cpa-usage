import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageModelMappingDistribution } from "@/types/api"
import type { UsageModelMappingSummary } from "@/features/usage-intelligence/model-mapping-summary"

function buildModelMappingsParams(provider: string, windowEnd = ""): URLSearchParams {
  const params = new URLSearchParams({ range: "24h" })
  if (provider) params.set("provider", provider)
  if (windowEnd) params.set("window_end", windowEnd)
  return params
}

export function buildModelMappingsPath(provider: string, windowEnd = ""): string {
  const params = buildModelMappingsParams(provider, windowEnd)
  return `/usage/model-mappings?${params.toString()}`
}

export function buildModelMappingsSummaryPath(provider: string): string {
  const params = buildModelMappingsParams(provider)
  return `/usage/model-mappings/summary?${params.toString()}`
}

export function useModelMappingsSummary(provider: string) {
  return useQuery({
    queryKey: ["usage", "model-mappings", "summary", "24h", provider],
    queryFn: ({ signal }) => apiFetch<UsageModelMappingSummary>(buildModelMappingsSummaryPath(provider), { signal }),
    staleTime: 60_000,
  })
}

export function useModelMappings(provider: string, enabled = true, windowEnd = "") {
  return useQuery({
    queryKey: ["usage", "model-mappings", "details", "24h", provider, windowEnd],
    queryFn: ({ signal }) => apiFetch<UsageModelMappingDistribution>(buildModelMappingsPath(provider, windowEnd), { signal }),
    enabled,
    staleTime: 60_000,
  })
}
