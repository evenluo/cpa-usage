import { createFileRoute } from "@tanstack/react-router"

export interface RequestsSearch {
  provider: string
  model: string
  modelAlias: string
  account: string
  endpoint: string
  status: string
  requestId: string
  windowEnd: string
  result: "" | "success" | "failed"
}

export function normalizeRequestsSearch(search: Record<string, unknown>): RequestsSearch {
  const result = search.result === "success" || search.result === "failed" ? search.result : ""
  return {
    provider: typeof search.provider === "string" ? search.provider.trim() : "",
    model: typeof search.model === "string" ? search.model.trim() : "",
    modelAlias: typeof search.modelAlias === "string" ? search.modelAlias.trim() : "",
    account: typeof search.account === "string" ? search.account.trim() : "",
    endpoint: typeof search.endpoint === "string" ? search.endpoint.trim() : "",
    status: typeof search.status === "string" ? search.status.trim().toLowerCase() : "",
    requestId: typeof search.requestId === "string" ? search.requestId.trim() : "",
    windowEnd: typeof search.windowEnd === "string" ? search.windowEnd.trim() : "",
    result,
  }
}

export const Route = createFileRoute("/requests")({
  validateSearch: normalizeRequestsSearch,
})
