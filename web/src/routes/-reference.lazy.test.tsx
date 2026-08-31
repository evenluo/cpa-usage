import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { useReferenceDataWorkbench, type UseReferenceDataWorkbenchResult } from "@/features/reference-data/use-reference-data-workbench"
import { ReferencePage } from "./reference.lazy"

vi.mock("@tanstack/react-router", () => ({
  createLazyFileRoute: () => (config: unknown) => config,
}))
vi.mock("@/features/reference-data/use-reference-data-workbench", () => ({
  useReferenceDataWorkbench: vi.fn(),
}))

const retryAPIKeys = vi.fn()
const retryAccounts = vi.fn()
const retryPricing = vi.fn()

function workbenchResult(overrides: Partial<UseReferenceDataWorkbenchResult> = {}): UseReferenceDataWorkbenchResult {
  return {
    query: "",
    setQuery: vi.fn(),
    keyAliasScope: "api-key",
    selectKeyAliasScope: vi.fn(),
    scopeDescription: "Human-readable labels for raw API keys",
    apiKeyCount: undefined,
    accountCount: 1,
    aliasedAPIKeys: undefined,
    aliasedAccounts: 1,
    missingRates: 0,
    apiKeysRead: { status: "error", retry: retryAPIKeys },
    accountsRead: { status: "ready", retry: retryAccounts },
    pricingRead: { status: "ready", retry: retryPricing },
    aliasRead: { status: "error", retry: retryAPIKeys },
    filteredKeys: [],
    editingId: null,
    draftAlias: "",
    setDraftAlias: vi.fn(),
    startEdit: vi.fn(),
    cancelEdit: vi.fn(),
    saveEdit: vi.fn(),
    clearEdit: vi.fn(),
    models: ["gpt-5"],
    pricingMap: new Map(),
    getDraft: vi.fn(() => ({ prompt: "", completion: "", cache: "" })),
    updateDraft: vi.fn(),
    saveRate: vi.fn(),
    savingModel: null,
    ...overrides,
  }
}

const apiKeyRow = {
  id: "sk-alpha",
  alias: "Agent API Key",
  displayName: "Agent API Key",
  name: "",
  identity: "sk-a…pha",
  provider: "OpenAI",
  type: "api-key",
  auth_type_name: "apikey",
  total_tokens: 17,
  total_cost: 1.5,
  cost_available: true,
  last_used_at: null,
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useReferenceDataWorkbench).mockReturnValue(workbenchResult())
})

afterEach(cleanup)

describe("ReferencePage read states", () => {
  it("keeps sibling panels truthful and retries only the failed API-key read", async () => {
    const user = userEvent.setup()
    render(<ReferencePage />)

    expect(screen.getByText("Failed to load API keys")).toBeInTheDocument()
    expect(screen.getByText("gpt-5")).toBeInTheDocument()
    expect(screen.getByText("1 aliased")).toBeInTheDocument()
    expect(screen.getByText("Unavailable")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Retry key aliases" }))
    expect(retryAPIKeys).toHaveBeenCalledTimes(1)
    expect(retryAccounts).not.toHaveBeenCalled()
    expect(retryPricing).not.toHaveBeenCalled()
  })

  it("renders an Accounts failure without masking API Keys or Cost Rates and retries only Accounts", async () => {
    const user = userEvent.setup()
    vi.mocked(useReferenceDataWorkbench).mockReturnValue(workbenchResult({
      keyAliasScope: "account",
      scopeDescription: "Human-readable labels for account keys",
      apiKeyCount: 1,
      aliasedAPIKeys: 1,
      apiKeysRead: { status: "ready", retry: retryAPIKeys },
      accountCount: undefined,
      aliasedAccounts: undefined,
      accountsRead: { status: "error", retry: retryAccounts },
      aliasRead: { status: "error", retry: retryAccounts },
    }))

    render(<ReferencePage />)

    expect(screen.getByText("Failed to load accounts")).toBeInTheDocument()
    expect(screen.queryByText("No keys found")).not.toBeInTheDocument()
    expect(screen.getByText("gpt-5")).toBeInTheDocument()
    expect(screen.getByText("1 aliased")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Retry key aliases" }))
    expect(retryAccounts).toHaveBeenCalledTimes(1)
    expect(retryAPIKeys).not.toHaveBeenCalled()
    expect(retryPricing).not.toHaveBeenCalled()
  })

  it("renders real empty Accounts while preserving API Keys and Cost Rates", () => {
    vi.mocked(useReferenceDataWorkbench).mockReturnValue(workbenchResult({
      keyAliasScope: "account",
      scopeDescription: "Human-readable labels for account keys",
      apiKeyCount: 1,
      aliasedAPIKeys: 1,
      apiKeysRead: { status: "ready", retry: retryAPIKeys },
      accountCount: 0,
      aliasedAccounts: 0,
      accountsRead: { status: "empty", retry: retryAccounts },
      aliasRead: { status: "empty", retry: retryAccounts },
    }))

    render(<ReferencePage />)

    expect(screen.getByText("No keys found")).toBeInTheDocument()
    expect(screen.queryByText("Failed to load accounts")).not.toBeInTheDocument()
    expect(screen.getByText("gpt-5")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Retry key aliases" })).not.toBeInTheDocument()
  })

  it("renders a Cost Rates failure without masking key data and retries only Cost Rates", async () => {
    const user = userEvent.setup()
    vi.mocked(useReferenceDataWorkbench).mockReturnValue(workbenchResult({
      apiKeyCount: 1,
      accountCount: 1,
      aliasedAPIKeys: 1,
      aliasedAccounts: 1,
      apiKeysRead: { status: "ready", retry: retryAPIKeys },
      accountsRead: { status: "ready", retry: retryAccounts },
      pricingRead: { status: "error", retry: retryPricing },
      aliasRead: { status: "ready", retry: retryAPIKeys },
      missingRates: undefined,
      filteredKeys: [apiKeyRow],
      models: [],
    }))

    render(<ReferencePage />)

    expect(screen.getByText("Failed to load cost rates")).toBeInTheDocument()
    expect(screen.queryByText("No models available for cost rates")).not.toBeInTheDocument()
    expect(screen.getByText("Agent API Key")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Retry cost rates" }))
    expect(retryPricing).toHaveBeenCalledTimes(1)
    expect(retryAPIKeys).not.toHaveBeenCalled()
    expect(retryAccounts).not.toHaveBeenCalled()
  })

  it("renders real empty Cost Rates while preserving key data", () => {
    vi.mocked(useReferenceDataWorkbench).mockReturnValue(workbenchResult({
      apiKeyCount: 1,
      accountCount: 1,
      aliasedAPIKeys: 1,
      aliasedAccounts: 1,
      apiKeysRead: { status: "ready", retry: retryAPIKeys },
      accountsRead: { status: "ready", retry: retryAccounts },
      pricingRead: { status: "empty", retry: retryPricing },
      aliasRead: { status: "ready", retry: retryAPIKeys },
      missingRates: 0,
      filteredKeys: [apiKeyRow],
      models: [],
    }))

    render(<ReferencePage />)

    expect(screen.getByText("No models available for cost rates")).toBeInTheDocument()
    expect(screen.queryByText("Failed to load cost rates")).not.toBeInTheDocument()
    expect(screen.getByText("Agent API Key")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Retry cost rates" })).not.toBeInTheDocument()
  })
})
