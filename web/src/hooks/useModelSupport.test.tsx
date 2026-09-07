import { act, renderHook, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ReactNode } from "react"
import { apiFetch } from "@/lib/api"
import { loadModelSupport, MODEL_SUPPORT_MAX_ACCOUNTS, useModelSupport } from "./useModelSupport"

vi.mock("@/lib/api", () => ({ apiFetch: vi.fn() }))

const mockedApiFetch = vi.mocked(apiFetch)

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient({ defaultOptions: { mutations: { retry: false } } })}>{children}</QueryClientProvider>
}

describe("model support explicit load", () => {
  beforeEach(() => mockedApiFetch.mockReset())

  it("does not issue a request on render and posts only after Load", async () => {
    mockedApiFetch.mockResolvedValue({ scope_complete: true, selected_count: 1, loaded_count: 1, accounts: [], models: [], limits: {} })
    const { result } = renderHook(() => useModelSupport(), { wrapper })

    expect(mockedApiFetch).not.toHaveBeenCalled()
    act(() => result.current.mutate([7]))

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(mockedApiFetch).toHaveBeenCalledTimes(1)
    expect(mockedApiFetch).toHaveBeenCalledWith("/usage/identities/model-support", {
      method: "POST",
      body: JSON.stringify({ identity_ids: [7] }),
    })
  })

  it("rejects oversize scope locally without silent truncation or a request", async () => {
    const ids = Array.from({ length: MODEL_SUPPORT_MAX_ACCOUNTS + 1 }, (_, index) => index + 1)

    await expect(loadModelSupport(ids)).rejects.toThrow("Narrow selection")
    expect(mockedApiFetch).not.toHaveBeenCalled()
  })
})
