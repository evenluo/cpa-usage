import { beforeEach, describe, expect, it, vi } from "vitest"
import { apiFetch } from "@/lib/api"
import { fetchSession } from "./useAuth"

vi.mock("@/lib/api", () => ({ apiFetch: vi.fn() }))

const mockedApiFetch = vi.mocked(apiFetch)

beforeEach(() => {
  mockedApiFetch.mockReset()
})

describe("fetchSession", () => {
  it.each([true, false])("accepts an explicit authenticated=%s response", async (authenticated) => {
    mockedApiFetch.mockResolvedValueOnce({ authenticated })
    await expect(fetchSession()).resolves.toEqual({ authenticated })
  })

  it("preserves transport and HTTP failures as errors", async () => {
    mockedApiFetch
      .mockRejectedValueOnce(new TypeError("Failed to fetch"))
      .mockRejectedValueOnce(new Error("API error 503"))

    await expect(fetchSession()).rejects.toThrow("Failed to fetch")
    await expect(fetchSession()).rejects.toThrow("API error 503")
  })

  it.each([{}, { authenticated: null }, { authenticated: 0 }, null])(
    "rejects malformed session payload %#",
    async (payload) => {
      mockedApiFetch.mockResolvedValueOnce(payload)
      await expect(fetchSession()).rejects.toThrow("Session response is malformed")
    },
  )
})
