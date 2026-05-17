import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { UsageEventsPage } from "@/types/api"

export function useEvents(range: string = "24h", pageSize: number = 20) {
  return useQuery({
    queryKey: ["events", range, pageSize],
    queryFn: () =>
      apiFetch<UsageEventsPage>(`/usage/events?range=${range}&page_size=${pageSize}`),
    staleTime: 30_000,
  })
}
