import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { RequestHealthResponse } from "@/types/api"

export function buildRequestHealthPath(range: string, provider: string): string {
  const params = new URLSearchParams({ range })
  if (provider) params.set("provider", provider)
  return `/usage/request-health?${params.toString()}`
}

export function useRequestHealth(range: string, provider: string) {
  return useQuery({
    queryKey: ["usage", "request-health", range, provider],
    queryFn: () => apiFetch<RequestHealthResponse>(buildRequestHealthPath(range, provider)),
    staleTime: 30_000,
    refetchInterval: () => {
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return false
      }
      return 60_000
    },
  })
}
