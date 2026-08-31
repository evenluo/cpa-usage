import { beforeEach, describe, expect, it, vi } from "vitest"
import type { KeyIdentity } from "@/types/api"
import { apiFetch } from "@/lib/api"
import {
  QUOTA_REFRESH_LIMIT,
  fetchAllAuthFileIdentities,
  quotaCacheQueryKey,
  resolveRefreshTaskUpdates,
  selectRefreshAuthIndexes,
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
    total_tokens: 0,
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

  it("keys the cache query by provider and auth-file identities, not analysis range or granularity", () => {
    const identities = [identity("b-auth", "Codex"), identity("a-auth", "Codex")]

    expect(quotaCacheQueryKey("Codex", identities)).toEqual([
      "quota",
      "cache",
      "Codex",
      "a-auth|b-auth",
    ])
    expect(quotaCacheQueryKey("Claude", identities)).toEqual([
      "quota",
      "cache",
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
        quota: { id: "b-auth", quota: [] },
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
    expect(result.hasCompleted).toBe(true)
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
