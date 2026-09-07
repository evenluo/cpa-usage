import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { renderHook, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { createElement, type ReactNode } from "react"
import { apiFetch, metricsFetch } from "@/lib/api"
import { MANUAL_SYNC_READ_MODEL_QUERY_KEYS, useMetrics } from "./useStatus"

vi.mock("@/lib/api", () => ({
  apiFetch: vi.fn(),
  metricsFetch: vi.fn(),
}))

const mockedAPIFetch = vi.mocked(apiFetch)
const mockedMetricsFetch = vi.mocked(metricsFetch)

afterEach(() => {
  mockedAPIFetch.mockReset()
  mockedMetricsFetch.mockReset()
})

describe("manual sync invalidation", () => {
  it("refreshes usage, evidence, identity, and reference-data read models", () => {
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).toEqual([
      ["analytics"],
      ["usage"],
      ["events"],
      ["keys"],
      ["pricing"],
    ])
  })

  it("covers both identity lists through the keys prefix", () => {
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).toContainEqual(["keys"])
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).not.toContainEqual(["keys", "identities"])
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).not.toContainEqual(["keys", "api-keys"])
  })
})

describe("useMetrics", () => {
  it("reads only the runtime metrics snapshot when Operations mounts", async () => {
    mockedMetricsFetch.mockResolvedValueOnce({
      redis_inbox_pending: 2,
      poller_running: true,
      poller_sync_running: false,
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client }, children)

    const { result } = renderHook(() => useMetrics(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(mockedMetricsFetch).toHaveBeenCalledTimes(1)
    expect(mockedAPIFetch).not.toHaveBeenCalled()
  })
})
