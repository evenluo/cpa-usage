import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import apiKeyAliasTargetsFixture from "@/test/contracts/api_key_alias_targets_page.json"
import usageIdentitiesFixture from "@/test/contracts/usage_identities_page.json"
import type { APIKeyAliasTarget, KeyIdentity } from "@/types/api"
import { apiFetch } from "@/lib/api"
import { fetchAllAPIKeys, fetchAllKeys } from "./useKeys"

vi.mock("@/lib/api", () => ({
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
    total_tokens: sample.total_tokens,
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
    cached_tokens: sample.cached_tokens,
    total_tokens: sample.total_tokens,
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

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
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
    mockedApiFetch.mockResolvedValueOnce({
      identities: [only],
      total_pages: 1,
    })

    await expect(fetchAllKeys()).resolves.toEqual([only])
    expect(mockedApiFetch).toHaveBeenCalledTimes(1)
    expect(pageQuery(String(mockedApiFetch.mock.calls[0][0]))).toEqual({
      resource: "/usage/identities/page",
      page: 1,
      pageSize: 100,
    })
  })

  it("treats a missing, zero, or sub-1 total_pages as a single page", async () => {
    const only = identity(7)
    mockedApiFetch
      .mockResolvedValueOnce({ identities: [only] })
      .mockResolvedValueOnce({ identities: [only], total_pages: 0 })
      .mockResolvedValueOnce({ identities: [only], total_pages: 0.9 })

    await expect(fetchAllKeys()).resolves.toEqual([only])
    await expect(fetchAllKeys()).resolves.toEqual([only])
    await expect(fetchAllKeys()).resolves.toEqual([only])
    expect(mockedApiFetch.mock.calls.map((call) => pageQuery(String(call[0])).page)).toEqual([1, 1, 1])
  })

  it("merges every remaining page after the first", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      return { identities: [identity(page)], total_pages: 3 }
    })

    await expect(fetchAllKeys()).resolves.toEqual([identity(1), identity(2), identity(3)])
    expect(mockedApiFetch.mock.calls.map((call) => pageQuery(String(call[0])).page).sort()).toEqual([1, 2, 3])
  })

  it("requests remaining pages concurrently after the first page resolves", async () => {
    const first = deferred<{ identities: KeyIdentity[]; total_pages: number }>()
    const second = deferred<{ identities: KeyIdentity[]; total_pages: number }>()
    const third = deferred<{ identities: KeyIdentity[]; total_pages: number }>()
    const pages = [first, second, third]

    mockedApiFetch.mockImplementation((path) => pages[pageQuery(String(path)).page - 1].promise)

    const result = fetchAllKeys()
    await vi.waitFor(() => expect(mockedApiFetch).toHaveBeenCalledTimes(1))
    expect(pageQuery(String(mockedApiFetch.mock.calls[0][0])).page).toBe(1)

    first.resolve({ identities: [identity(1)], total_pages: 3 })
    await vi.waitFor(() => expect(mockedApiFetch).toHaveBeenCalledTimes(3))
    expect(mockedApiFetch.mock.calls.slice(1).map((call) => pageQuery(String(call[0])).page).sort()).toEqual([2, 3])

    second.resolve({ identities: [identity(2)], total_pages: 3 })
    third.resolve({ identities: [identity(3)], total_pages: 3 })
    await expect(result).resolves.toEqual([identity(1), identity(2), identity(3)])
  })

  it("stops at advertised total_pages even when later pages would have more rows", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      if (page > 2) {
        throw new Error(`page ${page} must not be requested when total_pages is 2`)
      }
      return { identities: [identity(page)], total_pages: 2 }
    })

    await expect(fetchAllKeys()).resolves.toEqual([identity(1), identity(2)])
    expect(mockedApiFetch).toHaveBeenCalledTimes(2)
  })

  it("still requests advertised pages when they come back empty", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      return {
        identities: page === 1 ? [identity(1)] : [],
        total_pages: 3,
      }
    })

    await expect(fetchAllKeys()).resolves.toEqual([identity(1)])
    expect(mockedApiFetch.mock.calls.map((call) => pageQuery(String(call[0])).page).sort()).toEqual([1, 2, 3])
  })

  it("truncates a fractional total_pages before requesting remaining pages", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      if (page > 2) {
        throw new Error(`page ${page} must not be requested when total_pages truncates to 2`)
      }
      return { identities: [identity(page)], total_pages: 2.9 }
    })

    await expect(fetchAllKeys()).resolves.toEqual([identity(1), identity(2)])
    expect(mockedApiFetch).toHaveBeenCalledTimes(2)
  })

  it("returns an empty list when the only page has no identities", async () => {
    mockedApiFetch.mockResolvedValueOnce({ identities: [], total_pages: 1 })
    await expect(fetchAllKeys()).resolves.toEqual([])
  })

  it("treats a missing identities array as empty", async () => {
    mockedApiFetch.mockResolvedValueOnce({ total_pages: 1 })
    await expect(fetchAllKeys()).resolves.toEqual([])
  })

  it("rejects when the first page fails", async () => {
    mockedApiFetch.mockRejectedValueOnce(new Error("identities unavailable"))
    await expect(fetchAllKeys()).rejects.toThrow("identities unavailable")
    expect(mockedApiFetch).toHaveBeenCalledTimes(1)
  })

  it("rejects when any remaining page fails", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      if (page === 2) throw new Error("page 2 failed")
      return { identities: [identity(page)], total_pages: 3 }
    })

    await expect(fetchAllKeys()).rejects.toThrow("page 2 failed")
    expect(mockedApiFetch.mock.calls.map((call) => pageQuery(String(call[0])).page).sort()).toEqual([1, 2, 3])
  })
})

describe("fetchAllAPIKeys", () => {
  it("returns a single page of API keys without requesting further pages", async () => {
    const only = apiKey("key-a")
    mockedApiFetch.mockResolvedValueOnce({
      api_keys: [only],
      total_pages: 1,
    })

    await expect(fetchAllAPIKeys()).resolves.toEqual([only])
    expect(pageQuery(String(mockedApiFetch.mock.calls[0][0]))).toEqual({
      resource: "/usage/api-keys/page",
      page: 1,
      pageSize: 100,
    })
  })

  it("merges API key pages and ignores data beyond advertised total_pages", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      const { page } = pageQuery(String(path))
      if (page > 2) {
        throw new Error(`page ${page} must not be requested when total_pages is 2`)
      }
      return { api_keys: [apiKey(`key-${page}`)], total_pages: 2 }
    })

    await expect(fetchAllAPIKeys()).resolves.toEqual([apiKey("key-1"), apiKey("key-2")])
    expect(mockedApiFetch).toHaveBeenCalledTimes(2)
  })

  it("returns an empty list when api_keys is missing", async () => {
    mockedApiFetch.mockResolvedValueOnce({ total_pages: 1 })
    await expect(fetchAllAPIKeys()).resolves.toEqual([])
  })

  it("rejects when an API key page fails", async () => {
    mockedApiFetch.mockRejectedValueOnce(new Error("api keys unavailable"))
    await expect(fetchAllAPIKeys()).rejects.toThrow("api keys unavailable")
  })
})
