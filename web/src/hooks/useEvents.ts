import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageEventsPage } from "@/types/api"

export function useEvents(range: string = "24h", pageSize: number = 20, provider: string = "", page: number = 1) {
  const params = new URLSearchParams({ range, page_size: String(pageSize), page: String(page) })
  if (provider) params.set("provider", provider)

  return useQuery({
    queryKey: ["events", range, pageSize, provider, page],
    queryFn: () =>
      apiFetch<UsageEventsPage>(`/usage/events?${params.toString()}`),
    staleTime: 30_000,
    refetchInterval: () => {
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return false
      }
      return 60_000
    },
  })
}
