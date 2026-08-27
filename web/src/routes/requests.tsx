import { createFileRoute } from "@tanstack/react-router"

export interface RequestsSearch {
  provider: string
}

export function normalizeRequestsSearch(search: Record<string, unknown>): RequestsSearch {
  return {
    provider: typeof search.provider === "string" ? search.provider.trim() : "",
  }
}

export const Route = createFileRoute("/requests")({
  validateSearch: normalizeRequestsSearch,
})
