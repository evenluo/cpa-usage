import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import { validatePaginatedPage } from "@/lib/pagination"
import type { UsageDiagnosticSelection, UsageEventsPage } from "@/types/api"

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

export function buildEventsPath(
  range: string,
  pageSize: number,
  provider: string,
  page: number,
  filters: UsageDiagnosticSelection & { result?: "" | "success" | "failed" },
): string {
  const params = new URLSearchParams({ range, page_size: String(pageSize), page: String(page) })
  if (provider) params.set("provider", provider)
  if (filters.model) params.set("model", filters.model)
  if (filters.account) params.set("account", filters.account)
  if (filters.endpoint) params.set("endpoint", filters.endpoint)
  if (filters.status) params.set("status", filters.status)
  if (filters.requestId) params.set("request_id", filters.requestId)
  if (filters.windowEnd) params.set("window_end", filters.windowEnd)
  if (filters.result) params.set("result", filters.result)
  return `/usage/events?${params.toString()}`
}

export function useEvents(
  range: string = "24h",
  pageSize: number = 20,
  provider: string = "",
  page: number = 1,
  refetchInterval: number | false = 60_000,
  filters: UsageDiagnosticSelection & { result?: "" | "success" | "failed" } = {},
) {
  const path = buildEventsPath(range, pageSize, provider, page, filters)

  return useQuery({
    queryKey: ["events", range, pageSize, provider, page, filters.model || "", filters.account || "", filters.endpoint || "", filters.status || "", filters.requestId || "", filters.windowEnd || "", filters.result || ""],
    queryFn: () => fetchEvents(path, page, pageSize),
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
