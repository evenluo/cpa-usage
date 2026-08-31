import { createFileRoute } from "@tanstack/react-router"

export interface RequestsSearch {
  provider: string
  model: string
  result: "" | "success" | "failed"
}

export function normalizeRequestsSearch(search: Record<string, unknown>): RequestsSearch {
  const result = search.result === "success" || search.result === "failed" ? search.result : ""
  return {
    provider: typeof search.provider === "string" ? search.provider.trim() : "",
    model: typeof search.model === "string" ? search.model.trim() : "",
    result,
  }
}

export const Route = createFileRoute("/requests")({
  validateSearch: normalizeRequestsSearch,
})
