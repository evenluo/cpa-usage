import { act, renderHook } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { useToast } from "@/components/providers/toast-provider"
import { useAPIKeys, useDeleteAPIKeyAlias, useDeleteAlias, useKeys, useUpdateAPIKeyAlias, useUpdateAlias } from "@/hooks/useKeys"
import { usePricing, useSavePricing } from "@/hooks/usePricing"
import type { APIKeyAliasTarget, KeyIdentity, PricingEntry } from "@/types/api"
import { normalizeAccountKeyRows, normalizeAPIKeyRows } from "./model"
import { useReferenceDataWorkbench } from "./use-reference-data-workbench"

vi.mock("@/components/providers/toast-provider", () => ({
  useToast: vi.fn(),
}))
vi.mock("@/hooks/useKeys", () => ({
  useKeys: vi.fn(),
  useAPIKeys: vi.fn(),
  useUpdateAlias: vi.fn(),
  useUpdateAPIKeyAlias: vi.fn(),
  useDeleteAlias: vi.fn(),
  useDeleteAPIKeyAlias: vi.fn(),
}))
vi.mock("@/hooks/usePricing", () => ({
  usePricing: vi.fn(),
  useSavePricing: vi.fn(),
}))

const apiKey: APIKeyAliasTarget = {
  id: "sk-alpha",
  identity: "sk-alpha-trace",
  displayName: "sk-alpha…trace",
  alias: "Alpha Alias",
  provider: "OpenAI",
  auth_type: 2,
  auth_type_name: "apikey",
  total_requests: 3,
  success_count: 2,
  failure_count: 1,
  input_tokens: 10,
  output_tokens: 5,
  reasoning_tokens: 0,
  cached_tokens: 2,
  total_tokens: 17,
  total_cost: 1.5,
  cost_available: true,
  cost_status: "available",
  first_used_at: null,
  last_used_at: null,
}

const accountKey: KeyIdentity = {
  id: 7,
  name: "Account Name",
  displayName: "Account Display",
  alias: "Account Alias",
  auth_type: 1,
  auth_type_name: "oauth",
  identity: "acct-trace",
  type: "claude",
  provider: "Anthropic",
  total_tokens: 25,
  total_cost: 2,
  cost_available: false,
  last_used_at: "2026-05-18T00:00:00Z",
}

const configuredPricing: PricingEntry = {
  model: "configured-model",
  prompt_price_per_1m: 1,
  completion_price_per_1m: 2,
  cache_price_per_1m: 0.5,
}

const toast = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
}

const updateAPIKeyAlias = { mutateAsync: vi.fn() }
const updateAlias = { mutateAsync: vi.fn() }
const deleteAPIKeyAlias = { mutateAsync: vi.fn() }
const deleteAlias = { mutateAsync: vi.fn() }
const savePricing = { mutateAsync: vi.fn() }

function mockWorkbenchSources() {
  vi.mocked(useToast).mockReturnValue(toast)
  vi.mocked(useAPIKeys).mockReturnValue({ data: [apiKey], isLoading: false, error: null, refetch: vi.fn() } as never)
  vi.mocked(useKeys).mockReturnValue({ data: [accountKey], isLoading: false, error: null, refetch: vi.fn() } as never)
  vi.mocked(usePricing).mockReturnValue({
    data: { pricing: [configuredPricing], usedModels: ["missing-model", "configured-model"] },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  } as never)
  vi.mocked(useUpdateAPIKeyAlias).mockReturnValue(updateAPIKeyAlias as never)
  vi.mocked(useUpdateAlias).mockReturnValue(updateAlias as never)
  vi.mocked(useDeleteAPIKeyAlias).mockReturnValue(deleteAPIKeyAlias as never)
  vi.mocked(useDeleteAlias).mockReturnValue(deleteAlias as never)
  vi.mocked(useSavePricing).mockReturnValue(savePricing as never)
}

describe("useReferenceDataWorkbench", () => {
  beforeEach(() => {
    mockWorkbenchSources()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it("starts on the API Key alias scope with derived summary counts and models", () => {
    const { result } = renderHook(() => useReferenceDataWorkbench())

    expect(result.current.keyAliasScope).toBe("api-key")
    expect(result.current.scopeDescription).toBe("Human-readable labels for raw API keys")
    expect(result.current.filteredKeys).toEqual(normalizeAPIKeyRows([apiKey]))
    expect(result.current.apiKeyCount).toBe(1)
    expect(result.current.accountCount).toBe(1)
    expect(result.current.aliasedAPIKeys).toBe(1)
    expect(result.current.aliasedAccounts).toBe(1)
    expect(result.current.missingRates).toBe(1)
    expect(result.current.apiKeysRead.status).toBe("ready")
    expect(result.current.accountsRead.status).toBe("ready")
    expect(result.current.pricingRead.status).toBe("ready")
    expect(result.current.models).toEqual(["missing-model", "configured-model"])
    expect(result.current.editingId).toBeNull()
    expect(result.current.savingModel).toBeNull()
  })

  it("keeps API Keys, Accounts, and Cost Rates read state independent with scoped retries", () => {
    const retryAPIKeys = vi.fn()
    const retryAccounts = vi.fn()
    const retryPricing = vi.fn()
    vi.mocked(useAPIKeys).mockReturnValue({ data: undefined, isLoading: false, error: new Error("api keys failed"), refetch: retryAPIKeys } as never)
    vi.mocked(useKeys).mockReturnValue({ data: [accountKey], isLoading: false, error: null, refetch: retryAccounts } as never)
    vi.mocked(usePricing).mockReturnValue({ data: { pricing: [configuredPricing], usedModels: ["configured-model"] }, isLoading: false, error: null, refetch: retryPricing } as never)

    const { result } = renderHook(() => useReferenceDataWorkbench())

    expect(result.current.apiKeysRead.status).toBe("error")
    expect(result.current.apiKeyCount).toBeUndefined()
    expect(result.current.accountsRead.status).toBe("ready")
    expect(result.current.accountCount).toBe(1)
    expect(result.current.pricingRead.status).toBe("ready")
    act(() => result.current.apiKeysRead.retry())
    expect(retryAPIKeys).toHaveBeenCalledTimes(1)
    expect(retryAccounts).not.toHaveBeenCalled()
    expect(retryPricing).not.toHaveBeenCalled()
  })

  it("keeps an initial Accounts failure independent and retries only Accounts", () => {
    const retryAPIKeys = vi.fn()
    const retryAccounts = vi.fn()
    const retryPricing = vi.fn()
    vi.mocked(useAPIKeys).mockReturnValue({ data: [apiKey], isLoading: false, error: null, refetch: retryAPIKeys } as never)
    vi.mocked(useKeys).mockReturnValue({ data: undefined, isLoading: false, error: new Error("accounts failed"), refetch: retryAccounts } as never)
    vi.mocked(usePricing).mockReturnValue({ data: { pricing: [configuredPricing], usedModels: ["configured-model"] }, isLoading: false, error: null, refetch: retryPricing } as never)

    const { result } = renderHook(() => useReferenceDataWorkbench())

    expect(result.current.accountsRead.status).toBe("error")
    expect(result.current.accountCount).toBeUndefined()
    expect(result.current.apiKeysRead.status).toBe("ready")
    expect(result.current.apiKeyCount).toBe(1)
    expect(result.current.pricingRead.status).toBe("ready")
    act(() => result.current.accountsRead.retry())
    expect(retryAccounts).toHaveBeenCalledTimes(1)
    expect(retryAPIKeys).not.toHaveBeenCalled()
    expect(retryPricing).not.toHaveBeenCalled()
  })

  it("keeps an initial Cost Rates failure independent and retries only Cost Rates", () => {
    const retryAPIKeys = vi.fn()
    const retryAccounts = vi.fn()
    const retryPricing = vi.fn()
    vi.mocked(useAPIKeys).mockReturnValue({ data: [apiKey], isLoading: false, error: null, refetch: retryAPIKeys } as never)
    vi.mocked(useKeys).mockReturnValue({ data: [accountKey], isLoading: false, error: null, refetch: retryAccounts } as never)
    vi.mocked(usePricing).mockReturnValue({ data: undefined, isLoading: false, error: new Error("pricing failed"), refetch: retryPricing } as never)

    const { result } = renderHook(() => useReferenceDataWorkbench())

    expect(result.current.pricingRead.status).toBe("error")
    expect(result.current.missingRates).toBeUndefined()
    expect(result.current.apiKeysRead.status).toBe("ready")
    expect(result.current.accountsRead.status).toBe("ready")
    act(() => result.current.pricingRead.retry())
    expect(retryPricing).toHaveBeenCalledTimes(1)
    expect(retryAPIKeys).not.toHaveBeenCalled()
    expect(retryAccounts).not.toHaveBeenCalled()
  })

  it("reports a successful empty Accounts result while preserving API Keys and Cost Rates", () => {
    vi.mocked(useAPIKeys).mockReturnValue({ data: [apiKey], isLoading: false, error: null, refetch: vi.fn() } as never)
    vi.mocked(useKeys).mockReturnValue({ data: [], isLoading: false, error: null, refetch: vi.fn() } as never)
    vi.mocked(usePricing).mockReturnValue({ data: { pricing: [configuredPricing], usedModels: ["configured-model"] }, isLoading: false, error: null, refetch: vi.fn() } as never)

    const { result } = renderHook(() => useReferenceDataWorkbench())

    expect(result.current.accountsRead.status).toBe("empty")
    expect(result.current.accountCount).toBe(0)
    expect(result.current.apiKeysRead.status).toBe("ready")
    expect(result.current.pricingRead.status).toBe("ready")
  })

  it("distinguishes successful empty data and preserves cached data on refresh failure", () => {
    const staleError = new Error("refresh failed")
    vi.mocked(useAPIKeys).mockReturnValue({ data: [], isLoading: false, error: null, refetch: vi.fn() } as never)
    vi.mocked(useKeys).mockReturnValue({ data: [accountKey], isLoading: false, error: staleError, refetch: vi.fn() } as never)
    vi.mocked(usePricing).mockReturnValue({ data: { pricing: [], usedModels: [] }, isLoading: false, error: null, refetch: vi.fn() } as never)

    const { result } = renderHook(() => useReferenceDataWorkbench())

    expect(result.current.apiKeysRead.status).toBe("empty")
    expect(result.current.apiKeyCount).toBe(0)
    expect(result.current.accountsRead).toMatchObject({ status: "ready", refreshError: staleError })
    expect(result.current.accountCount).toBe(1)
    expect(result.current.pricingRead.status).toBe("empty")
    expect(result.current.missingRates).toBe(0)
  })

  it("filters the visible Key Alias rows and switches scope while clearing an in-progress edit", () => {
    const { result } = renderHook(() => useReferenceDataWorkbench())

    act(() => {
      result.current.startEdit(result.current.filteredKeys[0])
      result.current.setQuery("missing")
    })
    expect(result.current.editingId).toBe("sk-alpha")
    expect(result.current.draftAlias).toBe("Alpha Alias")
    expect(result.current.filteredKeys).toEqual([])

    act(() => {
      result.current.setQuery("")
      result.current.selectKeyAliasScope("account")
    })

    expect(result.current.keyAliasScope).toBe("account")
    expect(result.current.editingId).toBeNull()
    expect(result.current.filteredKeys).toEqual(normalizeAccountKeyRows([accountKey]))
    expect(result.current.scopeDescription).toBe("Human-readable labels for account keys")
  })

  it("cancelEdit leaves the current draft without saving", () => {
    const { result } = renderHook(() => useReferenceDataWorkbench())

    act(() => {
      result.current.startEdit(result.current.filteredKeys[0])
      result.current.setDraftAlias("Changed")
      result.current.cancelEdit()
    })

    expect(result.current.editingId).toBeNull()
    expect(result.current.draftAlias).toBe("Changed")
    expect(updateAPIKeyAlias.mutateAsync).not.toHaveBeenCalled()
  })

  it("rejects an empty Key Alias save and keeps the editor open", async () => {
    const { result } = renderHook(() => useReferenceDataWorkbench())

    act(() => {
      result.current.startEdit(result.current.filteredKeys[0])
      result.current.setDraftAlias("   ")
    })

    await act(async () => {
      await result.current.saveEdit(result.current.filteredKeys[0])
    })

    expect(toast.error).toHaveBeenCalledWith("Use clear to remove an alias")
    expect(updateAPIKeyAlias.mutateAsync).not.toHaveBeenCalled()
    expect(result.current.editingId).toBe("sk-alpha")
  })

  it("saves an API Key alias, then an account alias after switching scope", async () => {
    updateAPIKeyAlias.mutateAsync.mockResolvedValue(undefined)
    updateAlias.mutateAsync.mockResolvedValue(undefined)
    const { result } = renderHook(() => useReferenceDataWorkbench())

    act(() => {
      result.current.startEdit(result.current.filteredKeys[0])
      result.current.setDraftAlias("Ops Alias")
    })
    await act(async () => {
      await result.current.saveEdit(result.current.filteredKeys[0])
    })

    expect(updateAPIKeyAlias.mutateAsync).toHaveBeenCalledWith({ id: "sk-alpha", alias: "Ops Alias" })
    expect(toast.success).toHaveBeenCalledWith("Alias saved")
    expect(result.current.editingId).toBeNull()

    act(() => {
      result.current.selectKeyAliasScope("account")
    })
    act(() => {
      result.current.startEdit(result.current.filteredKeys[0])
      result.current.setDraftAlias("Account Ops")
    })
    await act(async () => {
      await result.current.saveEdit(result.current.filteredKeys[0])
    })

    expect(updateAlias.mutateAsync).toHaveBeenCalledWith({ id: 7, alias: "Account Ops" })
  })

  it("surfaces Key Alias save and clear mutation failures without closing the editor", async () => {
    updateAPIKeyAlias.mutateAsync.mockRejectedValue(new Error("save failed"))
    deleteAPIKeyAlias.mutateAsync.mockRejectedValue(new Error("clear failed"))
    const { result } = renderHook(() => useReferenceDataWorkbench())
    const row = result.current.filteredKeys[0]

    act(() => {
      result.current.startEdit(row)
    })
    await act(async () => {
      await result.current.saveEdit(row)
    })
    expect(toast.error).toHaveBeenCalledWith("Failed to save alias")
    expect(result.current.editingId).toBe("sk-alpha")

    await act(async () => {
      await result.current.clearEdit(row)
    })
    expect(toast.error).toHaveBeenCalledWith("Failed to clear alias")
    expect(result.current.editingId).toBe("sk-alpha")
  })

  it("clears Key Aliases for the active scope", async () => {
    deleteAPIKeyAlias.mutateAsync.mockResolvedValue(undefined)
    deleteAlias.mutateAsync.mockResolvedValue(undefined)
    const { result } = renderHook(() => useReferenceDataWorkbench())

    await act(async () => {
      await result.current.clearEdit(result.current.filteredKeys[0])
    })
    expect(deleteAPIKeyAlias.mutateAsync).toHaveBeenCalledWith("sk-alpha")
    expect(toast.success).toHaveBeenCalledWith("Alias cleared")

    act(() => {
      result.current.selectKeyAliasScope("account")
    })
    await act(async () => {
      await result.current.clearEdit(result.current.filteredKeys[0])
    })
    expect(deleteAlias.mutateAsync).toHaveBeenCalledWith(7)
  })

  it("rejects incomplete or invalid Cost Rate drafts before calling save", async () => {
    const { result } = renderHook(() => useReferenceDataWorkbench())

    await act(async () => {
      await result.current.saveRate("missing-model")
    })
    expect(toast.error).toHaveBeenCalledWith("Enter all rates before saving")
    expect(savePricing.mutateAsync).not.toHaveBeenCalled()

    act(() => {
      result.current.updateDraft("missing-model", "prompt", "1")
      result.current.updateDraft("missing-model", "completion", "-1")
      result.current.updateDraft("missing-model", "cache", "0")
    })
    await act(async () => {
      await result.current.saveRate("missing-model")
    })
    expect(toast.error).toHaveBeenCalledWith("Rates must be non-negative numbers")
    expect(savePricing.mutateAsync).not.toHaveBeenCalled()
  })

  it("saves a Cost Rate from the current draft and reports mutation failures", async () => {
    savePricing.mutateAsync.mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error("rate failed"))
    const { result } = renderHook(() => useReferenceDataWorkbench())

    expect(result.current.getDraft("configured-model")).toEqual({ prompt: "1", completion: "2", cache: "0.5" })
    act(() => {
      result.current.updateDraft("configured-model", "prompt", "3")
    })
    expect(result.current.getDraft("configured-model")).toEqual({ prompt: "3", completion: "2", cache: "0.5" })

    await act(async () => {
      await result.current.saveRate("configured-model")
    })
    expect(savePricing.mutateAsync).toHaveBeenCalledWith({
      model: "configured-model",
      prompt_price_per_1m: 3,
      completion_price_per_1m: 2,
      cache_price_per_1m: 0.5,
    })
    expect(toast.success).toHaveBeenCalledWith("configured-model cost rate saved")
    expect(result.current.savingModel).toBeNull()

    await act(async () => {
      await result.current.saveRate("configured-model")
    })
    expect(toast.error).toHaveBeenCalledWith("Failed to save cost rate")
    expect(result.current.savingModel).toBeNull()
  })
})
