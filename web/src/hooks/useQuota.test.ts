import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"
import { createElement, type ReactNode } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { KeyIdentity } from "@/types/api"
import { apiFetch } from "@/lib/api"
import {
  QUOTA_REFRESH_LIMIT,
  fetchAllAuthFileIdentities,
  mergeQuotaObservations,
  quotaObservationsQueryKey,
  resolveRefreshTaskUpdates,
  selectRefreshAuthIndexes,
  useLiveCapacity,
  type LiveCapacityTaskState,
} from "./useQuota"

vi.mock("@/lib/api", () => ({ apiFetch: vi.fn() }))

const mockedApiFetch = vi.mocked(apiFetch)

function identity(authIndex: string, provider: string): KeyIdentity {
  return {
    id: 1,
    name: authIndex,
    displayName: authIndex,
    alias: "",
    auth_type: 1,
    auth_type_name: "oauth",
    identity: authIndex,
    type: provider.toLowerCase(),
    provider,
    disabled: false,
    total_tokens: 0,
    canonical_valid_attempts: 0,
    total_cost: 0,
    cost_available: false,
    last_used_at: null,
  }
}

describe("quota hooks", () => {
  beforeEach(() => {
    mockedApiFetch.mockReset()
  })

  it("collects every auth-file identity page through the strict pagination contract", async () => {
    const identities = Array.from({ length: 101 }, (_, index) => identity(`auth-${index + 1}`, "Codex"))
    mockedApiFetch.mockImplementation(async (path) => {
      const page = Number(new URLSearchParams(String(path).split("?")[1]).get("page"))
      return {
        identities: page === 1 ? identities.slice(0, 100) : identities.slice(100),
        total_count: identities.length,
        page,
        page_size: 100,
        total_pages: 2,
      }
    })

    await expect(fetchAllAuthFileIdentities()).resolves.toEqual(identities)
    expect(mockedApiFetch).toHaveBeenCalledTimes(2)
  })

  it("rejects malformed auth-file pagination instead of exposing an empty identity set", async () => {
    mockedApiFetch.mockResolvedValueOnce({
      identities: [],
      total_count: 0,
      page: 1,
      page_size: 100,
      total_pages: 0,
    })

    await expect(fetchAllAuthFileIdentities()).rejects.toThrow("invalid total_pages")
  })

  it("keys the observation query by provider and auth-file identities, not analysis range or granularity", () => {
    const identities = [identity("b-auth", "Codex"), identity("a-auth", "Codex")]

    expect(quotaObservationsQueryKey("Codex", identities)).toEqual([
      "quota",
      "observations",
      "Codex",
      "a-auth|b-auth",
    ])
    expect(quotaObservationsQueryKey("Claude", identities)).toEqual([
      "quota",
      "observations",
      "Claude",
      "a-auth|b-auth",
    ])
  })

  it("marks an individual polling failure as failed without keeping the task active", async () => {
    const taskStates: Record<string, LiveCapacityTaskState> = {
      "a-auth": { status: "queued", taskId: "missing-task" },
      "b-auth": { status: "running", taskId: "completed-task" },
    }

    const result = await resolveRefreshTaskUpdates(taskStates, async (taskId) => {
      if (taskId === "missing-task") {
        throw new Error("API error 404")
      }
      return {
        taskId,
        authIndex: "b-auth",
        status: "completed",
        quota: { id: "b-auth", observedAt: "2026-09-08T10:00:00Z", quota: [] },
      }
    })

    expect(result.updates["a-auth"]).toMatchObject({
      status: "failed",
      taskId: "missing-task",
      error: "API error 404",
    })
    expect(result.updates["b-auth"]).toMatchObject({
      status: "completed",
      taskId: "completed-task",
    })
    expect(result.completedObservations).toEqual([
      { id: "b-auth", observedAt: "2026-09-08T10:00:00Z", quota: [] },
    ])
  })

  it("keeps a newer successful observation when an older server snapshot arrives", () => {
    const current = {
      items: [{ id: "a-auth", observedAt: "2026-09-08T10:00:00Z", quota: [{ key: "5h", usedPercent: 40 }] }],
    }
    const incoming = {
      items: [{ id: "a-auth", observedAt: "2026-09-08T09:00:00Z", quota: [{ key: "5h", usedPercent: 10 }] }],
    }

    expect(mergeQuotaObservations(current, incoming)).toEqual(current)
  })

  it("replaces an observation only when a later successful one arrives", () => {
    const current = {
      items: [{ id: "a-auth", observedAt: "2026-09-08T09:00:00Z", quota: [{ key: "5h", usedPercent: 10 }] }],
    }
    const incoming = {
      items: [{ id: "a-auth", observedAt: "2026-09-08T10:00:00Z", quota: [{ key: "5h", usedPercent: 40 }] }],
    }

    expect(mergeQuotaObservations(current, incoming)).toEqual(incoming)
  })

  it("keeps the current observation when timestamps are equal", () => {
    const current = {
      items: [{ id: "a-auth", observedAt: "2026-09-08T10:00:00Z", quota: [{ key: "5h", usedPercent: 40 }] }],
    }
    const incoming = {
      items: [{ id: "a-auth", observedAt: "2026-09-08T10:00:00Z", quota: [{ key: "5h", usedPercent: 10 }] }],
    }

    expect(mergeQuotaObservations(current, incoming)).toEqual(current)
  })

  it("caps refresh-all selection to the backend refresh limit", () => {
    const requestedAuthIndexes = Array.from({ length: QUOTA_REFRESH_LIMIT + 5 }, (_, index) => `auth-${index + 1}`)

    expect(selectRefreshAuthIndexes({ requestedAuthIndexes, taskStates: {} })).toEqual(
      requestedAuthIndexes.slice(0, QUOTA_REFRESH_LIMIT),
    )
  })

  it("skips active rows before applying the refresh-all cap", () => {
    const requestedAuthIndexes = Array.from({ length: QUOTA_REFRESH_LIMIT + 2 }, (_, index) => `auth-${index + 1}`)
    const selected = selectRefreshAuthIndexes({
      requestedAuthIndexes,
      taskStates: {
        "auth-1": { status: "running", taskId: "task-1" },
        "auth-2": { status: "starting" },
      },
    })

    expect(selected).toHaveLength(QUOTA_REFRESH_LIMIT)
    expect(selected).not.toContain("auth-1")
    expect(selected).not.toContain("auth-2")
    expect(selected[selected.length - 1]).toBe(`auth-${QUOTA_REFRESH_LIMIT + 2}`)
  })
})

describe("useLiveCapacity refresh targeting", () => {
  const identities = [
    identity("a-auth", "Codex"),
    identity("b-auth", "Claude"),
    identity("c-auth", "Codex"),
  ]

  beforeEach(() => {
    mockedApiFetch.mockReset()
    mockedApiFetch.mockImplementation(async (path) => {
      if (String(path).startsWith("/usage/identities/page")) {
        return { identities, total_count: identities.length, page: 1, page_size: 100, total_pages: 1 }
      }
      if (path === "/quota/observations") return { items: [] }
      if (path === "/quota/refresh") {
        return { tasks: [], rejected: [], accepted: 0, skipped: 0, limit: QUOTA_REFRESH_LIMIT }
      }
      throw new Error(`unexpected path: ${String(path)}`)
    })
  })

  function renderLiveCapacity(enabled = true) {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return renderHook(() => useLiveCapacity("", enabled), {
      wrapper: ({ children }: { children: ReactNode }) =>
        createElement(QueryClientProvider, { client }, children),
    })
  }

  it("does not scan identities before an offscreen card is enabled", async () => {
    const { result } = renderLiveCapacity(false)

    await Promise.resolve()

    expect(result.current.identities).toEqual([])
    expect(mockedApiFetch).not.toHaveBeenCalled()
  })

  async function dispatchedRefreshIndexes(): Promise<string[][]> {
    await waitFor(() => {
      expect(mockedApiFetch.mock.calls.some(([path]) => path === "/quota/refresh")).toBe(true)
    })
    return mockedApiFetch.mock.calls
      .filter(([path]) => path === "/quota/refresh")
      .map(([, init]) => JSON.parse(String(init?.body)).auth_indexes as string[])
  }

  it("refreshes every visible identity when no target is given", async () => {
    const { result } = renderLiveCapacity()
    await waitFor(() => expect(result.current.identities).toHaveLength(3))

    act(() => { result.current.refresh() })

    expect(await dispatchedRefreshIndexes()).toEqual([["a-auth", "b-auth", "c-auth"]])
  })

  it("limits a single-string target to that one account", async () => {
    const { result } = renderLiveCapacity()
    await waitFor(() => expect(result.current.identities).toHaveLength(3))

    act(() => { result.current.refresh("b-auth") })

    expect(await dispatchedRefreshIndexes()).toEqual([["b-auth"]])
  })

  it("refreshes exactly the given auth-index list", async () => {
    const { result } = renderLiveCapacity()
    await waitFor(() => expect(result.current.identities).toHaveLength(3))

    act(() => { result.current.refresh(["a-auth", "c-auth"]) })

    expect(await dispatchedRefreshIndexes()).toEqual([["a-auth", "c-auth"]])
  })

  it("publishes a completed observation immediately and preserves it across an older snapshot and later failure", async () => {
    let refreshCallCount = 0
    let observationReadCount = 0
    mockedApiFetch.mockImplementation(async (path) => {
      if (String(path).startsWith("/usage/identities/page")) {
        return { identities, total_count: identities.length, page: 1, page_size: 100, total_pages: 1 }
      }
      if (path === "/quota/observations") {
        observationReadCount += 1
        return {
          items: [{
            id: "a-auth",
            observedAt: "2026-09-08T09:00:00Z",
            quota: [{ key: "5h", label: "5h", usedPercent: 10 }],
          }],
        }
      }
      if (path === "/quota/refresh") {
        refreshCallCount += 1
        if (refreshCallCount === 1) {
          return {
            tasks: [{ authIndex: "a-auth", taskId: "task-a" }],
            rejected: [],
            accepted: 1,
            skipped: 0,
            limit: QUOTA_REFRESH_LIMIT,
          }
        }
        throw new Error("refresh unavailable")
      }
      if (path === "/quota/refresh/task-a") {
        return {
          taskId: "task-a",
          authIndex: "a-auth",
          status: "completed",
          quota: {
            id: "a-auth",
            observedAt: "2026-09-08T10:00:00Z",
            quota: [{ key: "5h", label: "5h", usedPercent: 40 }],
          },
        }
      }
      throw new Error(`unexpected path: ${String(path)}`)
    })
    const { result } = renderLiveCapacity()
    await waitFor(() => expect(result.current.observations?.items[0].quota[0].usedPercent).toBe(10))

    act(() => { result.current.refresh("a-auth") })
    await waitFor(() => {
      expect(result.current.observations?.items[0]).toMatchObject({
        observedAt: "2026-09-08T10:00:00Z",
        quota: [{ usedPercent: 40 }],
      })
    })
    await waitFor(() => expect(observationReadCount).toBeGreaterThan(1))

    act(() => { result.current.refresh("a-auth") })
    await waitFor(() => expect(result.current.taskStates["a-auth"]?.status).toBe("failed"))
    expect(result.current.observations?.items[0]).toMatchObject({
      observedAt: "2026-09-08T10:00:00Z",
      quota: [{ usedPercent: 40 }],
    })
  })

  it("excludes disabled accounts from refresh-all and single-target refreshes", async () => {
    const disabledIdentity: KeyIdentity = { ...identity("d-auth", "Codex"), disabled: true }
    mockedApiFetch.mockImplementation(async (path) => {
      if (String(path).startsWith("/usage/identities/page")) {
        return { identities: [...identities, disabledIdentity], total_count: 4, page: 1, page_size: 100, total_pages: 1 }
      }
      if (path === "/quota/observations") return { items: [] }
      if (path === "/quota/refresh") {
        return { tasks: [], rejected: [], accepted: 0, skipped: 0, limit: QUOTA_REFRESH_LIMIT }
      }
      throw new Error(`unexpected path: ${String(path)}`)
    })
    const { result } = renderLiveCapacity()
    await waitFor(() => expect(result.current.identities).toHaveLength(4))

    act(() => { result.current.refresh() })
    expect(await dispatchedRefreshIndexes()).toEqual([["a-auth", "b-auth", "c-auth"]])

    await act(async () => { result.current.refresh("d-auth") })
    expect(mockedApiFetch.mock.calls.filter(([path]) => path === "/quota/refresh")).toHaveLength(1)
  })
})
