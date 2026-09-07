import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageModelMappingDistribution } from "@/types/api"

export function buildModelMappingsPath(provider: string): string {
  const params = new URLSearchParams({ range: "24h" })
  if (provider) params.set("provider", provider)
  return `/usage/model-mappings?${params.toString()}`
}

export function useModelMappings(provider: string) {
  return useQuery({
    queryKey: ["usage", "model-mappings", "24h", provider],
    queryFn: () => apiFetch<UsageModelMappingDistribution>(buildModelMappingsPath(provider)),
    staleTime: 30_000,
  })
}
