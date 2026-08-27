import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { useReferenceDataWorkbench } from "@/features/reference-data/use-reference-data-workbench"
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

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useReferenceDataWorkbench).mockReturnValue({
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
  })
})

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
})
