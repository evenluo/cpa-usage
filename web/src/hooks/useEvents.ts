import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import { validatePaginatedPage } from "@/lib/pagination"
import type { UsageEventsPage } from "@/types/api"

export async function fetchEvents(path: string, page: number, pageSize: number): Promise<UsageEventsPage> {
  const payload = await apiFetch<UsageEventsPage>(path)
  validatePaginatedPage<UsageEventsPage, UsageEventsPage["events"][number]>({
    payload,
    expectedPage: page,
    resource: "Request evidence",
    expectedPageSize: pageSize,
    getItems: (response) => response.events,
  })
  return payload
}

export function useEvents(
  range: string = "24h",
  pageSize: number = 20,
  provider: string = "",
  page: number = 1,
  refetchInterval: number | false = 60_000,
) {
  const params = new URLSearchParams({ range, page_size: String(pageSize), page: String(page) })
  if (provider) params.set("provider", provider)

  return useQuery({
    queryKey: ["events", range, pageSize, provider, page],
    queryFn: () => fetchEvents(`/usage/events?${params.toString()}`, page, pageSize),
    staleTime: 30_000,
    refetchInterval: () => {
      if (refetchInterval === false) return false
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return false
      }
      return refetchInterval
    },
  })
}
