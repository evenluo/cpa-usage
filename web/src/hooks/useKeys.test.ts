import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"
import { createElement, type ReactNode } from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import apiKeyAliasTargetsFixture from "@/test/contracts/api_key_alias_targets_page.json"
import usageIdentitiesFixture from "@/test/contracts/usage_identities_page.json"
import type { APIKeyAliasTarget, APIKeyAliasTargetPage, KeyIdentity, KeyIdentityPage } from "@/types/api"
import { ApiError, apiFetch } from "@/lib/api"
import { fetchAllAPIKeys, fetchAllKeys, useSetIdentityDisabled } from "./useKeys"

vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  apiFetch: vi.fn(),
}))

const mockedApiFetch = vi.mocked(apiFetch)

function identity(id: number): KeyIdentity {
  const sample = usageIdentitiesFixture.identities[0]
  return {
    id,
    name: `key-${id}`,
    displayName: `key-${id}`,
    alias: sample.alias,
    auth_type: sample.auth_type,
    auth_type_name: sample.auth_type_name,
    identity: `id-${id}`,
    type: sample.type,
    provider: sample.provider,
    disabled: false,
    total_tokens: sample.total_tokens,
    canonical_valid_attempts: sample.canonical_valid_attempts,
    total_cost: sample.total_cost,
    cost_available: sample.cost_available,
    last_used_at: sample.last_used_at,
  }
}

function apiKey(id: string): APIKeyAliasTarget {
  const sample = apiKeyAliasTargetsFixture.api_keys[0]
  return {
    id,
    identity: id,
    displayName: id,
    alias: sample.alias,
    provider: sample.provider,
    auth_type: sample.auth_type,
    auth_type_name: sample.auth_type_name,
    total_requests: sample.total_requests,
    success_count: sample.success_count,
    failure_count: sample.failure_count,
    input_tokens: sample.input_tokens,
    output_tokens: sample.output_tokens,
    reasoning_tokens: sample.reasoning_tokens,
    cache_read_tokens: sample.cache_read_tokens,
    total_tokens: sample.total_tokens,
    canonical_valid_attempts: sample.canonical_valid_attempts,
    total_cost: sample.total_cost,
    cost_available: sample.cost_available,
    cost_status: "available",
    first_used_at: sample.first_used_at,
    last_used_at: sample.last_used_at,
  }
}

function pageQuery(path: string): { resource: string; page: number; pageSize: number } {
  const [pathname, search = ""] = path.split("?")
  const params = new URLSearchParams(search)
  return {
    resource: pathname,
    page: Number(params.get("page")),
    pageSize: Number(params.get("page_size")),
  }
}

function identityPage(page: number, totalCount: number, identities: KeyIdentity[]): KeyIdentityPage {
  return {
    identities,
    total_count: totalCount,
    page,
    page_size: 100,
    total_pages: Math.max(1, Math.ceil(totalCount / 100)),
  }
}

function apiKeyPage(page: number, totalCount: number, apiKeys: APIKeyAliasTarget[]): APIKeyAliasTargetPage {
  return {
    api_keys: apiKeys,
    total_count: totalCount,
    page,
    page_size: 100,
    total_pages: Math.max(1, Math.ceil(totalCount / 100)),
  }
}

beforeEach(() => {
  mockedApiFetch.mockReset()
})

afterEach(() => {
  vi.clearAllMocks()
})

describe("fetchAllKeys", () => {
  it("returns a single page without requesting further pages", async () => {
    const only = identity(1)
    mockedApiFetch.mockResolvedValueOnce(identityPage(1, 1, [only]))

    await expect(fetchAllKeys()).resolves.toEqual([only])
    expect(mockedApiFetch).toHaveBeenCalledTimes(1)
    expect(pageQuery(String(mockedApiFetch.mock.calls[0][0]))).toEqual({
      resource: "/usage/identities/page",
      page: 1,
      pageSize: 100,
    })
  })

  it.each([
    ["missing", undefined],
    ["zero", 0],
    ["fractional", 0.9],
    ["negative", -1],
    ["non-finite", Number.POSITIVE_INFINITY],
  ])("rejects %s total_pages metadata", async (_name, totalPages) => {
    mockedApiFetch.mockResolvedValueOnce({ ...identityPage(1, 1, [identity(7)]), total_pages: totalPages })

    await expect(fetchAllKeys()).rejects.toThrow("invalid total_pages")
  })

  it("merges every remaining page after the first", async () => {
    const rows = Array.from({ length: 101 }, (_, index) => identity(index + 1))
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      return page === 1
        ? identityPage(1, rows.length, rows.slice(0, 100))
        : identityPage(2, rows.length, rows.slice(100))
    })

    await expect(fetchAllKeys()).resolves.toEqual(rows)
    expect(mockedApiFetch).toHaveBeenCalledTimes(2)
  })

  it("rejects rather than returning a partial collection when a later page is empty", async () => {
    const firstPage = Array.from({ length: 100 }, (_, index) => identity(index + 1))
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      return identityPage(page, 101, page === 1 ? firstPage : [])
    })

    await expect(fetchAllKeys()).rejects.toThrow("incomplete page item count")
  })

  it("returns an empty list when the only page has no identities", async () => {
    mockedApiFetch.mockResolvedValueOnce(identityPage(1, 0, []))
    await expect(fetchAllKeys()).resolves.toEqual([])
  })

  it("rejects a missing identities array as malformed", async () => {
    mockedApiFetch.mockResolvedValueOnce({ ...identityPage(1, 0, []), identities: undefined })
    await expect(fetchAllKeys()).rejects.toThrow("invalid items")
  })

  it("rejects when the first page fails", async () => {
    mockedApiFetch.mockRejectedValueOnce(new Error("identities unavailable"))
    await expect(fetchAllKeys()).rejects.toThrow("identities unavailable")
    expect(mockedApiFetch).toHaveBeenCalledTimes(1)
  })

  it("rejects when any remaining page fails", async () => {
    const firstPage = Array.from({ length: 100 }, (_, index) => identity(index + 1))
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      if (page === 2) throw new Error("page 2 failed")
      return identityPage(page, 101, firstPage)
    })

    await expect(fetchAllKeys()).rejects.toThrow("page 2 failed")
    expect(mockedApiFetch.mock.calls.map((call) => pageQuery(String(call[0])).page).sort()).toEqual([1, 2])
  })
})

describe("fetchAllAPIKeys", () => {
  it("returns a single page of API keys without requesting further pages", async () => {
    const only = apiKey("key-a")
    mockedApiFetch.mockResolvedValueOnce(apiKeyPage(1, 1, [only]))

    await expect(fetchAllAPIKeys()).resolves.toEqual([only])
    expect(pageQuery(String(mockedApiFetch.mock.calls[0][0]))).toEqual({
      resource: "/usage/api-keys/page",
      page: 1,
      pageSize: 100,
    })
  })

  it("merges API key pages and ignores data beyond advertised total_pages", async () => {
    const rows = Array.from({ length: 101 }, (_, index) => apiKey(`key-${index + 1}`))
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      return page === 1
        ? apiKeyPage(1, rows.length, rows.slice(0, 100))
        : apiKeyPage(2, rows.length, rows.slice(100))
    })

    await expect(fetchAllAPIKeys()).resolves.toEqual(rows)
    expect(mockedApiFetch).toHaveBeenCalledTimes(2)
  })

  it("returns a normalized empty API key page and rejects missing items", async () => {
    mockedApiFetch.mockResolvedValueOnce(apiKeyPage(1, 0, []))
    await expect(fetchAllAPIKeys()).resolves.toEqual([])

    mockedApiFetch.mockResolvedValueOnce({ ...apiKeyPage(1, 0, []), api_keys: undefined })
    await expect(fetchAllAPIKeys()).rejects.toThrow("invalid items")
  })

  it("rejects when an API key page fails", async () => {
    mockedApiFetch.mockRejectedValueOnce(new Error("api keys unavailable"))
    await expect(fetchAllAPIKeys()).rejects.toThrow("api keys unavailable")
  })
})

describe("useSetIdentityDisabled", () => {
  function setupMutation() {
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    const invalidateQueries = vi.spyOn(client, "invalidateQueries").mockResolvedValue(undefined)
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children)
    const hook = renderHook(() => useSetIdentityDisabled(), { wrapper })
    return { ...hook, invalidateQueries }
  }

  it("reloads both identity queries after a successful toggle", async () => {
    mockedApiFetch.mockResolvedValueOnce({ disabled: true })
    const { result, invalidateQueries } = setupMutation()

    await act(async () => {
      await result.current.mutateAsync({ id: 7, disabled: true })
    })

    await waitFor(() => expect(invalidateQueries).toHaveBeenCalledTimes(2))
    expect(invalidateQueries.mock.calls.map(([filters]) => filters)).toEqual([
      { queryKey: ["quota", "auth-file-identities"] },
      { queryKey: ["keys", "identities"] },
    ])
  })

  it("reloads both identity queries after a failed toggle", async () => {
    mockedApiFetch.mockRejectedValueOnce(new ApiError(
      502,
      JSON.stringify({
        error: "Saved, but CPA status is unconfirmed. Trigger Sync.",
      }),
    ))
    const { result, invalidateQueries } = setupMutation()

    await act(async () => {
      await expect(result.current.mutateAsync({ id: 9, disabled: false })).rejects.toThrow(
        "Saved, but CPA status is unconfirmed. Trigger Sync.",
      )
    })

    await waitFor(() => expect(invalidateQueries).toHaveBeenCalledTimes(2))
    expect(invalidateQueries.mock.calls.map(([filters]) => filters)).toEqual([
      { queryKey: ["quota", "auth-file-identities"] },
      { queryKey: ["keys", "identities"] },
    ])
  })

  it.each([
    ["invalid JSON", "not-json"],
    ["a non-string error", JSON.stringify({ error: { message: "unsafe shape" } })],
  ])("uses the generic message for %s in an API error body", async (_case, body) => {
    mockedApiFetch.mockRejectedValueOnce(new ApiError(502, body))
    const { result } = setupMutation()

    await act(async () => {
      await expect(result.current.mutateAsync({ id: 9, disabled: false })).rejects.toThrow("Failed to update account")
    })
  })

  it("keeps network failure details out of the account toast while preserving the cause", async () => {
    const networkError = new Error("Failed to fetch")
    mockedApiFetch.mockRejectedValueOnce(networkError)
    const { result } = setupMutation()

    await act(async () => {
      await expect(result.current.mutateAsync({ id: 9, disabled: false })).rejects.toMatchObject({
        message: "Failed to update account",
        cause: networkError,
      })
    })
  })
})
