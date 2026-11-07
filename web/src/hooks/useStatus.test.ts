import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { createElement, type ReactNode } from "react"
import { apiFetch, metricsFetch } from "@/lib/api"
import { useLiveCapacity } from "./useQuota"
import { MANUAL_SYNC_READ_MODEL_QUERY_KEYS, useManualSync, useMetrics } from "./useStatus"

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
      ["quota", "auth-file-identities"],
    ])
  })

  it("rereads Live Capacity account metadata after sync without refreshing provider quota or tasks", async () => {
    let synced = false
    mockedAPIFetch.mockImplementation(async (path) => {
      if (path === "/sync") { synced = true; return {} }
      if (String(path).startsWith("/usage/identities/page")) return {
        identities: [{
          id: 1, name: "account", displayName: "Account", alias: "", auth_type: 1,
          auth_type_name: "oauth", identity: "account", type: "codex", provider: "Codex",
          disabled: false, total_tokens: 0, canonical_valid_attempts: 0,
          total_cost: 0, cost_available: false, last_used_at: null,
          auth_renewal: { requires_sign_in: synced },
        }],
        total_count: 1, page: 1, page_size: 100, total_pages: 1,
      }
      if (path === "/quota/observations") return {
        items: [{ id: "account", observedAt: "2026-11-08T00:00:00Z", quota: [{ key: "credits", balance: "1000" }] }],
      }
      throw new Error(`unexpected request ${String(path)}`)
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const taskKey = ["quota", "refresh-tasks", "terminal-task"]
    client.setQueryData(taskKey, { status: "completed" })
    const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client }, children)
    const { result } = renderHook(() => ({ capacity: useLiveCapacity(""), sync: useManualSync() }), { wrapper })
    await waitFor(() => expect(result.current.capacity.observations?.items).toHaveLength(1))
    expect(result.current.capacity.identities[0].auth_renewal?.requires_sign_in).toBe(false)

    await act(async () => { await result.current.sync.mutateAsync() })
    await waitFor(() => expect(result.current.capacity.identities[0].auth_renewal?.requires_sign_in).toBe(true))
    expect(mockedAPIFetch.mock.calls.filter(([path]) => String(path).startsWith("/usage/identities/page"))).toHaveLength(2)
    expect(mockedAPIFetch.mock.calls.filter(([path]) => path === "/quota/observations")).toHaveLength(1)
    expect(mockedAPIFetch.mock.calls.some(([path]) => String(path).startsWith("/quota/refresh"))).toBe(false)
    expect(client.getQueryState(taskKey)?.isInvalidated).toBe(false)
    expect(result.current.capacity.observations?.items[0].observedAt).toBe("2026-11-08T00:00:00Z")
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
