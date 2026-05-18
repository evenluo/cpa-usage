import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageEventsPage } from "@/types/api"

export function useEvents(range: string = "24h", pageSize: number = 20, provider: string = "") {
  const params = new URLSearchParams({ range, page_size: String(pageSize) })
  if (provider) params.set("provider", provider)

  return useQuery({
    queryKey: ["events", range, pageSize, provider],
    queryFn: () =>
      apiFetch<UsageEventsPage>(`/usage/events?${params.toString()}`),
    staleTime: 30_000,
  })
}
