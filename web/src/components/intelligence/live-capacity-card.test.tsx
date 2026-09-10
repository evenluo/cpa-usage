import { act } from "react"
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { KeyIdentity, ModelSupportResponse, QuotaObservationsResponse } from "@/types/api"
import type { LiveCapacityTaskState } from "@/hooks/useQuota"
import { formatDate } from "@/lib/format"

const OBSERVED_AT = "2026-09-07T09:00:00Z"

// Mock the useLiveCapacity hook
const mockUseLiveCapacity = vi.fn()
vi.mock("@/hooks/useQuota", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/hooks/useQuota")>()
  return {
    ...original,
    useLiveCapacity: (...args: Parameters<typeof original.useLiveCapacity>) => mockUseLiveCapacity(...args),
  }
})

const mockUseModelSupport = vi.fn()
vi.mock("@/hooks/useModelSupport", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/hooks/useModelSupport")>()
  return {
    ...original,
    useModelSupport: () => mockUseModelSupport(),
  }
})

// Mock FLIP hook to avoid DOM measurement in jsdom
vi.mock("@/hooks/useFlipReorder", () => ({
  useFlipReorder: () => ({
    containerRef: { current: null },
    registerItem: () => () => {},
  }),
}))

// Mock the identity disabled mutation; tiles call it directly.
const mockSetIdentityDisabled = { mutate: vi.fn(), isPending: false }
vi.mock("@/hooks/useKeys", () => ({
  useSetIdentityDisabled: () => mockSetIdentityDisabled,
}))

const mockToast = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }
vi.mock("@/components/providers/toast-provider", () => ({
  useToast: () => mockToast,
}))

import { LiveCapacityCard } from "./live-capacity-card"

function identity(overrides: Partial<KeyIdentity>): KeyIdentity {
  return {
    id: 1,
    name: "codex-auth",
    displayName: "Codex Auth",
    alias: "",
    auth_type: 1,
    auth_type_name: "oauth",
    identity: "codex-auth",
    type: "codex",
    provider: "Codex",
    disabled: false,
    total_tokens: 0,
    canonical_valid_attempts: 0,
    total_cost: 0,
    cost_available: false,
    last_used_at: null,
    ...overrides,
  }
}

interface LiveCapacityReturn {
  identities: KeyIdentity[]
  observations: QuotaObservationsResponse | undefined
  taskStates: Record<string, LiveCapacityTaskState>
  refresh: (target?: string | string[]) => void
  refreshLimit: number
  isLoading: boolean
  isRefreshing: boolean
  error: unknown
}

function setupMock(props: Partial<LiveCapacityReturn> = {}): LiveCapacityReturn {
  const defaults: LiveCapacityReturn = {
    identities: [],
    observations: undefined,
    taskStates: {},
    refresh: vi.fn(),
    refreshLimit: 20,
    isLoading: false,
    isRefreshing: false,
    error: null,
  }
  const merged = { ...defaults, ...props }
  mockUseLiveCapacity.mockReturnValue(merged)
  mockUseModelSupport.mockReturnValue({
    mutate: vi.fn(),
    reset: vi.fn(),
    data: undefined,
    isPending: false,
    isError: false,
    error: null,
  })
  return merged
}

/** Select the section-level grids (not the MetricMeter inner grids). */
function getSectionGrids(container: Element): Element[] {
  return Array.from(container.querySelectorAll(".grid.grid-cols-1"))
}

/** Read authIndex list from a grid element's children (full value lives on data-auth-index). */
function readGridAuthIndexes(grid: Element): string[] {
  return Array.from(grid.children).map((child) => {
    return child.querySelector("[data-auth-index]")?.getAttribute("data-auth-index") ?? ""
  })
}

describe("LiveCapacityCard", () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it("defers account reads until the card approaches the viewport", () => {
    let notifyIntersection: IntersectionObserverCallback | undefined
    class MockIntersectionObserver {
      constructor(callback: IntersectionObserverCallback) {
        notifyIntersection = callback
      }
      observe = vi.fn()
      disconnect = vi.fn()
      unobserve = vi.fn()
      takeRecords = () => []
      root = null
      rootMargin = "240px 0px"
      thresholds = [0]
    }
    vi.stubGlobal("IntersectionObserver", MockIntersectionObserver)
    setupMock({ isLoading: true })

    render(<LiveCapacityCard provider="" />)

    expect(mockUseLiveCapacity).toHaveBeenLastCalledWith("", false)
    expect(screen.getByText("Live Capacity")).toBeInTheDocument()
    expect(screen.queryByText("No auth-file accounts")).not.toBeInTheDocument()

    act(() => {
      notifyIntersection?.([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
    })
    expect(mockUseLiveCapacity).toHaveBeenLastCalledWith("", true)
  })

  it("shows skeleton while loading", () => {
    setupMock({ isLoading: true })
    const { container } = render(<LiveCapacityCard provider="" />)
    expect(container.querySelectorAll(".animate-pulse")).toHaveLength(3)
  })

  it("shows empty state when no identities exist", () => {
    setupMock()
    render(<LiveCapacityCard provider="" />)
    expect(screen.getByText("No auth-file accounts")).toBeInTheDocument()
  })

  it("keeps a read error distinct from an empty account list", () => {
    setupMock({ error: new Error("state read failed") })
    render(<LiveCapacityCard provider="" />)
    expect(screen.getByText("Failed to load live capacity")).toBeInTheDocument()
    expect(screen.queryByText("No auth-file accounts")).not.toBeInTheDocument()
  })

  it("shows independent account states and source timing", () => {
    setupMock({ identities: [identity({
      disabled: true,
      unavailable: true,
      status: "error",
      metadata_observed_at: "2026-09-07T08:00:00Z",
      last_refresh: "2026-09-07T07:45:00Z",
      next_retry_after: "2026-09-07T08:30:00Z",
    })] })
    render(<LiveCapacityCard provider="" />)

    const availability = screen.getByRole("group", { name: "Account availability" })
    expect(within(availability).getByText("Unavailable")).toBeInTheDocument()
    expect(within(availability).getByText("CPA: Error")).toBeInTheDocument()

    const timing = screen.getByRole("group", { name: "Account and observation timing" })
    for (const label of ["Metadata observed", "Token refreshed", "Retry eligible"]) {
      expect(within(timing).getByText(label)).toBeInTheDocument()
    }
    expect(within(timing).getByText("Retry eligible").closest("div")).toHaveAttribute(
      "title",
      "Eligibility time only, not a recovery guarantee.",
    )
    for (const timestamp of ["2026-09-07T08:00:00Z", "2026-09-07T07:45:00Z", "2026-09-07T08:30:00Z"]) {
      expect(timing.querySelector(`time[datetime='${timestamp}']`)).toBeInTheDocument()
    }
    expect(screen.queryByText(/^Last updated /)).not.toBeInTheDocument()
  })

  it("uses a model-only passive quota as the latest successful observation", () => {
    vi.useFakeTimers({ now: new Date("2026-09-07T12:00:00Z") })
    try {
      setupMock({
        identities: [identity({
          metadata_observed_at: "2026-09-07T11:00:00Z",
          passive_model_quotas: [{
            source: "cpa_passive",
            scope: "model",
            model: "gpt-5.3-codex",
            observed_at: "2026-09-07T09:00:00Z",
            quota: [{ key: "weekly", label: "Weekly", usedPercent: 20 }],
          }],
        })],
      })
      render(<LiveCapacityCard provider="" />)

      const updated = screen.getByText("Last updated 3h ago")
      expect(updated.parentElement).toHaveAttribute(
        "title",
        `Reported by CPA (gpt-5.3-codex) · ${formatDate("2026-09-07T09:00:00Z")}`,
      )
    } finally {
      vi.useRealTimers()
    }
  })

  it("renders account and model passive observations separately from a manual probe", () => {
    vi.useFakeTimers({ now: new Date("2026-09-07T08:00:00Z") })
    try {
      setupMock({
        identities: [identity({
          disabled: true,
          passive_quota: {
            source: "cpa_passive",
            scope: "account",
            observed_at: "2026-09-07T08:00:00Z",
            active_limit: "codex_bengalfox",
            quota: [
              { key: "primary", label: "5h", usedPercent: 25, allowed: false, resetAfterSeconds: 120, window: { seconds: 18_000 } },
              { key: "credits", label: "Credits", remaining: 4.5, unit: "credits" },
            ],
          },
          passive_model_quotas: [{
            source: "cpa_passive",
            scope: "model",
            model: "gpt-5.3-codex",
            observed_at: "2026-09-07T07:30:00Z",
            quota: [{ key: "secondary", label: "Weekly", allowed: false, window: { seconds: 604_800 } }],
          }],
        })],
        observations: {
          items: [{ id: "codex-auth", observedAt: "2026-09-07T09:00:00Z", quota: [{ key: "manual", label: "5h", usedPercent: 10 }] }],
        },
      })
      const { container } = render(<LiveCapacityCard provider="" />)

      // Disabled accounts retain historical readings and can refresh their quota.
      expect(screen.getByText("10% used")).toBeInTheDocument()
      expect(screen.getByLabelText("5h: 10% used")).toBeInTheDocument()
      expect(screen.getByText("No reading")).toBeInTheDocument()
      expect(screen.queryByText("25% used · Blocked")).not.toBeInTheDocument()
      expect(screen.getByRole("button", { name: "Refresh Codex Auth" })).toBeEnabled()
      // Window-less reported rows, the active limit, and model observations live
      // behind the per-tile fold.
      const fold = screen.getByText(/··· \d+ more/).closest("details")
      expect(fold).not.toHaveAttribute("open")
      expect(within(fold as HTMLElement).getByText("Active limit codex_bengalfox")).toBeInTheDocument()
      expect(within(fold as HTMLElement).getByText("4.5 credits left")).toBeInTheDocument()
      expect(within(fold as HTMLElement).getByText("Per-model quotas (1)")).toBeInTheDocument()
      expect(within(fold as HTMLElement).getByText("gpt-5.3-codex")).toBeInTheDocument()
      // Reported meters: folded Credits + folded model Weekly; the newer manual 5h wins its window.
      expect(container.querySelectorAll("div[title^='Reported by CPA · observed']")).toHaveLength(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it("merges probe and reported windows into one list and folds named limits and timing away", () => {
    vi.useFakeTimers({ now: new Date("2026-09-07T12:00:00Z") })
    try {
      setupMock({
        identities: [identity({
          metadata_observed_at: "2026-09-07T09:00:00Z",
          passive_quota: {
            source: "cpa_passive",
            scope: "account",
            observed_at: "2026-09-07T08:00:00Z",
            quota: [{ key: "passive-5h", label: "5h", usedPercent: 99, window: { seconds: 18_000 } }],
          },
        })],
        observations: {
          items: [{
            id: "codex-auth",
            observedAt: "2026-09-07T09:00:00Z",
            quota: [
              { key: "manual-5h", label: "5h", usedPercent: 10, window: { seconds: 18_000 } },
              { key: "spark", label: "GPT-5.3-Codex-Spark 5h", usedPercent: 20 },
            ],
          }],
        },
      })
      render(<LiveCapacityCard provider="" />)

      // One merged 5h meter: the newer probe reading wins, with no reset placeholder.
      expect(screen.getAllByLabelText(/^5h: /)).toHaveLength(1)
      expect(screen.getByText("10% used")).toBeInTheDocument()
      expect(screen.queryByText("99% used")).not.toBeInTheDocument()
      expect(screen.queryByText("-")).not.toBeInTheDocument()
      // The Weekly skeleton slot has no reading from either source.
      expect(screen.getByText("No reading")).toBeInTheDocument()
      // Freshness line: the newer of the two observation times, with both
      // sources and their absolute times on the tooltip.
      const updated = screen.getByText("Last updated 3h ago")
      expect(updated.parentElement).toHaveAttribute(
        "title",
        `Manual probe · ${formatDate("2026-09-07T09:00:00Z")}\nReported by CPA · ${formatDate("2026-09-07T08:00:00Z")}`,
      )
      // Named additional limits and timing lines live behind the fold; a shared
      // metadata/probe observation time collapses to a single "Observed" line.
      const fold = screen.getByText("··· 2 more").closest("details")
      expect(fold).not.toHaveAttribute("open")
      expect(within(fold as HTMLElement).getByText("More limits (1)")).toBeInTheDocument()
      expect(within(fold as HTMLElement).getByText("GPT-5.3-Codex-Spark 5h")).toBeInTheDocument()
      expect(within(fold as HTMLElement).getByText("Observed")).toBeInTheDocument()
      expect(within(fold as HTMLElement).queryByText("Metadata observed")).not.toBeInTheDocument()
      expect(within(fold as HTMLElement).queryByText(/cache expires|stale/i)).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it("does not present missing account state as active", () => {
    setupMock({ identities: [identity({ status: undefined, unavailable: undefined })] })
    render(<LiveCapacityCard provider="" />)
    expect(screen.queryByRole("group", { name: "Account availability" })).not.toBeInTheDocument()
    expect(screen.queryByText(/^CPA:/)).not.toBeInTheDocument()
    expect(screen.queryByText(/··· \d+ more/)).not.toBeInTheDocument()
  })

  it("renders tiles for each identity", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const observations: QuotaObservationsResponse = {
      items: [
        { id: "codex-pro", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, observations })
    render(<LiveCapacityCard provider="" />)

    expect(screen.getByText("Codex Pro")).toBeInTheDocument()
    expect(screen.getByText("Alpha Codex")).toBeInTheDocument()
  })

  it("loads model support only after an explicit selected-scope action", async () => {
    const user = userEvent.setup()
    const identities = [
      identity({ id: 1, identity: "codex-a", displayName: "Codex A" }),
      identity({ id: 2, identity: "codex-b", displayName: "Codex B" }),
    ]
    setupMock({ identities })
    const mutate = vi.fn()
    mockUseModelSupport.mockReturnValue({ mutate, reset: vi.fn(), data: undefined, isPending: false, isError: false, error: null })
    render(<LiveCapacityCard provider="" />)

    expect(mutate).not.toHaveBeenCalled()
    await user.click(screen.getByRole("button", { name: "Select displayed" }))
    await user.click(screen.getByRole("button", { name: "Load model support" }))

    expect(mutate).toHaveBeenCalledTimes(1)
    expect(mutate.mock.calls[0][0]).toEqual([1, 2])
  })

  it("clears only model-support state when the provider changes", async () => {
    const user = userEvent.setup()
    setupMock({ identities: [identity({ id: 1, identity: "codex-a", displayName: "Codex A" })] })
    const reset = vi.fn()
    mockUseModelSupport.mockReturnValue({ mutate: vi.fn(), reset, data: undefined, isPending: false, isError: false, error: null })
    const view = render(<LiveCapacityCard provider="Codex" />)

    await user.click(screen.getByRole("button", { name: "Select displayed" }))
    expect(screen.getByText("1/12 accounts selected")).toBeInTheDocument()
    reset.mockClear()

    view.rerender(<LiveCapacityCard provider="Gemini" />)

    await waitFor(() => expect(screen.getByText("0/12 accounts selected")).toBeInTheDocument())
    expect(reset).toHaveBeenCalledTimes(1)
  })

  it("shows partial registered support without a single-account conclusion", async () => {
    const user = userEvent.setup()
    const identities = [
      identity({ id: 1, identity: "codex-a", displayName: "Codex A" }),
      identity({ id: 2, identity: "codex-b", displayName: "Codex B" }),
    ]
    const data: ModelSupportResponse = {
      scope_complete: false,
      selected_count: 2,
      loaded_count: 1,
      accounts: [{
        identity_id: 1,
        auth_index: "codex-a",
        display_name: "Codex A",
        provider: "Codex",
        channel: "codex",
        disabled: false,
        unavailable: null,
        status: "loaded",
        catalog_status: "loaded",
        registered_models: [{
          id: "gpt-exact",
          definition_status: "available",
          capability: { context_length: 200000, supported_input_modalities: ["TEXT", "IMAGE"], thinking: { zero_allowed: false } },
        }],
      }, {
        identity_id: 2,
        auth_index: "codex-b",
        display_name: "Codex B",
        provider: "Codex",
        channel: "codex",
        disabled: true,
        unavailable: true,
        status: "failed",
        error_code: "upstream_error",
        catalog_status: "loaded",
        registered_models: [],
      }],
      models: [{
        model_id: "gpt-exact",
        observed_supporting_accounts: 1,
        selected_accounts: 2,
        single_registered_account_in_scope: null,
      }],
      limits: { max_accounts: 12, max_concurrency: 4, timeout_seconds: 15, max_upstream_requests: 36 },
    }
    setupMock({ identities })
    const mutate = vi.fn((_ids: number[], options: { onSuccess?: () => void }) => options.onSuccess?.())
    mockUseModelSupport.mockReturnValue({ mutate, reset: vi.fn(), data, isPending: false, isError: false, error: null })
    render(<LiveCapacityCard provider="" />)

    await user.click(screen.getByRole("button", { name: "Select displayed" }))
    await user.click(screen.getByRole("button", { name: "Load model support" }))

    expect(await screen.findByText("Partial selected scope")).toBeInTheDocument()
    expect(screen.getByText("observed in 1/1 loaded accounts")).toBeInTheDocument()
    expect(screen.queryByText(/1 registered account in this scope/)).not.toBeInTheDocument()
    expect(screen.getByText("Failed accounts are unknown, not unsupported")).toBeInTheDocument()
    expect(screen.getByText(/Context 200,000/)).toBeInTheDocument()
    expect(screen.getByText(/zero not allowed/)).toBeInTheDocument()
  })

  it("visualizes observation time, subscription window, and additional quota rows", () => {
    const futureUntil = new Date(Date.now() + 30 * 86_400_000).toISOString()
    const identities = [identity({
      identity: "codex-pro",
      displayName: "Codex Pro",
      provider: "Codex",
      type: "codex",
      active_start: "2026-08-01T00:00:00Z",
      active_until: futureUntil,
    })]
    const observations: QuotaObservationsResponse = {
      items: [{
        id: "codex-pro",
        observedAt: "2026-08-31T01:00:00Z",
        quota: [
          { key: "primary", label: "5h", usedPercent: 10 },
          { key: "secondary", label: "Weekly", usedPercent: 20 },
          { key: "review", label: "Code review", remaining: 15 },
        ],
      }],
    }
    setupMock({ identities, observations })
    render(<LiveCapacityCard provider="" />)

    expect(screen.getByText("Code review")).toBeInTheDocument()
    const timing = screen.getByRole("group", { name: "Account and observation timing" })
    expect(within(timing).getByText("Observed")).toBeInTheDocument()
    // Past subscription starts are hidden; only the future end date remains,
    // folded with the other timing lines.
    expect(within(timing).getByText("Ends")).toBeInTheDocument()
    expect(within(timing).queryByText("Starts")).not.toBeInTheDocument()
    expect(timing.querySelectorAll("time")).toHaveLength(2)
    expect(timing.querySelector("time[datetime='2026-08-31T01:00:00Z']")).toBeInTheDocument()
    expect(timing.querySelector(`time[datetime='${futureUntil}']`)).toBeInTheDocument()
    // Data freshness sits on the card surface instead of the subscription end.
    expect(screen.getByText(/^Last updated /)).toBeInTheDocument()
  })

  it("keeps an old observation visible without cache-expiry or stale UI", () => {
    const identities = [identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" })]
    const observations: QuotaObservationsResponse = {
      items: [{
        id: "codex-pro",
        observedAt: "2020-01-01T00:00:00Z",
        quota: [{ key: "primary", label: "5h", usedPercent: 10 }],
      }],
    }
    setupMock({ identities, observations })
    render(<LiveCapacityCard provider="" />)

    const timing = screen.getByRole("group", { name: "Account and observation timing" })
    expect(within(timing).getByText("Observed")).toBeInTheDocument()
    expect(screen.getByText("10% used")).toBeInTheDocument()
    expect(screen.getByText(/^Last updated /)).toBeInTheDocument()
    expect(screen.queryByText(/stale|cache expires|expired/i)).not.toBeInTheDocument()
  })

  it("truncates long auth indexes and copies the full value on click", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true })
    const longId = "9fa4210cc85a897c"
    const identities = [
      identity({ identity: longId, displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "short-id", displayName: "Short Id", provider: "Codex", type: "codex" }),
    ]
    const observations: QuotaObservationsResponse = {
      items: [
        { id: longId, observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10 }] },
        { id: "short-id", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10 }] },
      ],
    }
    setupMock({ identities, observations })
    render(<LiveCapacityCard provider="" />)

    const copyButton = screen.getByRole("button", { name: `Copy auth index ${longId}` })
    expect(copyButton).toHaveTextContent("9fa4210c…")
    expect(screen.getByRole("button", { name: "Copy auth index short-id" })).toHaveTextContent("short-id")

    fireEvent.click(copyButton)
    expect(writeText).toHaveBeenCalledWith(longId)
    await waitFor(() => expect(mockToast.success).toHaveBeenCalledWith("Auth index copied"))
  })

  it("folds the subscription end into the timing lines when the active start is missing", () => {
    const futureUntil = new Date(Date.now() + 30 * 86_400_000).toISOString()
    const identities = [identity({
      identity: "codex-pro",
      displayName: "Codex Pro",
      provider: "Codex",
      type: "codex",
      active_until: futureUntil,
    })]
    const observations: QuotaObservationsResponse = {
      items: [{
        id: "codex-pro",
        observedAt: OBSERVED_AT,
        quota: [{ key: "primary", label: "5h", usedPercent: 10 }],
      }],
    }
    setupMock({ identities, observations })
    render(<LiveCapacityCard provider="" />)

    const timing = screen.getByRole("group", { name: "Account and observation timing" })
    expect(within(timing).getByText("Ends")).toBeInTheDocument()
    expect(within(timing).queryByText("Starts")).not.toBeInTheDocument()
    expect(timing.querySelector(`time[datetime='${futureUntil}']`)).toBeInTheDocument()
    expect(screen.getByText(/^Last updated /)).toBeInTheDocument()
  })

  it("shows both subscription endpoints when the active start is still in the future", () => {
    const futureStart = new Date(Date.now() + 7 * 86_400_000).toISOString()
    const futureUntil = new Date(Date.now() + 37 * 86_400_000).toISOString()
    const identities = [identity({
      identity: "codex-pro",
      displayName: "Codex Pro",
      provider: "Codex",
      type: "codex",
      active_start: futureStart,
      active_until: futureUntil,
    })]
    const observations: QuotaObservationsResponse = {
      items: [{
        id: "codex-pro",
        observedAt: OBSERVED_AT,
        quota: [{ key: "primary", label: "5h", usedPercent: 10 }],
      }],
    }
    setupMock({ identities, observations })
    const { container } = render(<LiveCapacityCard provider="" />)

    const timing = within(container).getByRole("group", { name: "Account and observation timing" })
    expect(within(timing).getByText("Starts")).toBeInTheDocument()
    expect(within(timing).getByText("Ends")).toBeInTheDocument()
    expect(timing.querySelector(`time[datetime='${futureStart}']`)).toBeInTheDocument()
    expect(timing.querySelector(`time[datetime='${futureUntil}']`)).toBeInTheDocument()
  })

  it("separates priority accounts from regular accounts with a divider", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const observations: QuotaObservationsResponse = {
      items: [
        { id: "codex-pro", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, observations })
    const { container } = render(<LiveCapacityCard provider="" />)

    // Should have two grid sections (priority + regular)
    const grids = getSectionGrids(container)
    expect(grids).toHaveLength(2)

    // Priority grid: only codex-pro
    expect(readGridAuthIndexes(grids[0])).toEqual(["codex-pro"])
    // Regular grid: only plain-codex
    expect(readGridAuthIndexes(grids[1])).toEqual(["plain-codex"])

    // Divider should be present
    expect(container.querySelector("[role='separator']")).toBeInTheDocument()
  })

  it("shows no divider when all accounts are non-priority", () => {
    const identities = [
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
      identity({ identity: "team-codex", displayName: "Team Codex", provider: "Codex", type: "codex" }),
    ]
    const observations: QuotaObservationsResponse = {
      items: [
        { id: "plain-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "team-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, observations })
    const { container } = render(<LiveCapacityCard provider="" />)

    // Only one grid section (regular only)
    const grids = getSectionGrids(container)
    expect(grids).toHaveLength(1)

    // No divider
    expect(container.querySelector("[role='separator']")).not.toBeInTheDocument()
  })

  it("moves an account to the priority section when a later observation upgrades its plan", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const initialObservations: QuotaObservationsResponse = {
      items: [
        { id: "codex-pro", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, observations: initialObservations })
    const { container, rerender } = render(<LiveCapacityCard provider="" />)

    // Initially: 1 priority (codex-pro), 1 regular (plain-codex)
    let grids = getSectionGrids(container)
    expect(readGridAuthIndexes(grids[0])).toEqual(["codex-pro"])
    expect(readGridAuthIndexes(grids[1])).toEqual(["plain-codex"])

    // A later successful observation upgrades plain-codex to pro.
    setupMock({
      identities,
      observations: {
        items: [
          initialObservations.items[0],
          {
            id: "plain-codex",
            observedAt: "2026-09-07T10:00:00Z",
            quota: [{ key: "quota", label: "5h", usedPercent: 20, planType: "pro" }],
          },
        ],
      },
    })
    act(() => { rerender(<LiveCapacityCard provider="" />) })

    // Now both should be in the priority section, regular section empty
    grids = getSectionGrids(container)
    expect(grids).toHaveLength(1) // only priority grid
    // Both are Pro now (priority 0), sorted alphabetically: "Alpha Codex" < "Codex Pro"
    expect(readGridAuthIndexes(grids[0])).toEqual(["plain-codex", "codex-pro"])
    // No divider since regular section is empty
    expect(container.querySelector("[role='separator']")).not.toBeInTheDocument()
  })

  it("preserves regular section order when a later observation changes a reading", () => {
    const identities = [
      identity({ identity: "alpha-codex", displayName: "Alpha", provider: "Codex", type: "codex" }),
      identity({ identity: "beta-codex", displayName: "Beta", provider: "Codex", type: "codex" }),
    ]
    const observations: QuotaObservationsResponse = {
      items: [
        { id: "alpha-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "beta-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, observations })
    const { container, rerender } = render(<LiveCapacityCard provider="" />)

    const grids = getSectionGrids(container)
    const initialOrder = readGridAuthIndexes(grids[0])

    // A later beta-codex observation changes its reading without changing its plan.
    setupMock({
      identities,
      observations: {
        items: [
          observations.items[0],
          {
            id: "beta-codex",
            observedAt: "2026-09-07T10:00:00Z",
            quota: [{ key: "quota", label: "5h", usedPercent: 50, planType: "team" }],
          },
        ],
      },
    })
    act(() => { rerender(<LiveCapacityCard provider="" />) })

    const gridsAfter = getSectionGrids(container)
    expect(readGridAuthIndexes(gridsAfter[0])).toEqual(initialOrder)
  })

  it("filters tiles via provider chips and restores the full list on All", async () => {
    const user = userEvent.setup()
    const identities = [
      identity({
        identity: "codex-a",
        displayName: "Codex A",
        provider: "Codex",
        type: "codex",
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: "2026-09-07T08:00:00Z",
          quota: [{ key: "codex-passive", label: "Codex passive evidence", allowed: true }],
        },
      }),
      identity({ identity: "codex-b", displayName: "Codex B", provider: "Codex", type: "codex" }),
      identity({
        identity: "claude-a",
        displayName: "Claude A",
        provider: "Claude",
        type: "claude",
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: "2026-09-07T08:00:00Z",
          quota: [{ key: "claude-passive", label: "Claude passive evidence", allowed: true }],
        },
      }),
    ]
    const observations: QuotaObservationsResponse = {
      items: [
        { id: "codex-a", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "codex-b", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "claude-a", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, observations })
    render(<LiveCapacityCard provider="" />)

    const chipGroup = screen.getByRole("group", { name: "Filter accounts by provider" })
    const allChip = within(chipGroup).getByRole("button", { name: /^All/ })
    const codexChip = within(chipGroup).getByRole("button", { name: /Codex/ })
    const claudeChip = within(chipGroup).getByRole("button", { name: /Claude/ })
    expect(within(codexChip).getByText("2")).toBeInTheDocument()
    expect(within(claudeChip).getByText("1")).toBeInTheDocument()
    expect(allChip).toHaveAttribute("aria-pressed", "true")

    await user.click(claudeChip)
    expect(screen.queryByText("Codex A")).not.toBeInTheDocument()
    expect(screen.queryByText("Codex passive evidence")).not.toBeInTheDocument()
    expect(screen.getByText("Claude A")).toBeInTheDocument()
    expect(screen.getByText("Claude passive evidence")).toBeInTheDocument()
    expect(claudeChip).toHaveAttribute("aria-pressed", "true")

    await user.click(allChip)
    expect(screen.getByText("Codex A")).toBeInTheDocument()
    expect(screen.getByText("Codex passive evidence")).toBeInTheDocument()
    expect(screen.getByText("Claude A")).toBeInTheDocument()
  })

  it("hides provider chips when only one provider is present", () => {
    const identities = [
      identity({ identity: "codex-a", displayName: "Codex A", provider: "Codex", type: "codex" }),
    ]
    setupMock({ identities })
    render(<LiveCapacityCard provider="" />)

    expect(screen.queryByRole("group", { name: "Filter accounts by provider" })).not.toBeInTheDocument()
  })

  it("scopes the header refresh to the selected provider chip", async () => {
    const user = userEvent.setup()
    const identities = [
      identity({ identity: "codex-a", displayName: "Codex A", provider: "Codex", type: "codex" }),
      identity({ identity: "claude-a", displayName: "Claude A", provider: "Claude", type: "claude" }),
    ]
    const mock = setupMock({ identities })
    render(<LiveCapacityCard provider="" />)

    const chipGroup = screen.getByRole("group", { name: "Filter accounts by provider" })
    await user.click(within(chipGroup).getByRole("button", { name: /Claude/ }))
    await user.click(screen.getByRole("button", { name: "Refresh" }))
    expect(mock.refresh).toHaveBeenCalledWith(["claude-a"])

    await user.click(within(chipGroup).getByRole("button", { name: /^All/ }))
    await user.click(screen.getByRole("button", { name: "Refresh" }))
    expect(mock.refresh).toHaveBeenLastCalledWith()
  })

  it("preserves the regular section order after switching provider chips", async () => {
    const user = userEvent.setup()
    const identities = [
      identity({ identity: "alpha", displayName: "Alpha", provider: "Codex", type: "codex" }),
      identity({ identity: "beta", displayName: "Beta", provider: "Claude", type: "claude" }),
      identity({ identity: "gamma", displayName: "Gamma", provider: "Codex", type: "codex" }),
    ]
    const observations: QuotaObservationsResponse = {
      items: [
        { id: "alpha", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "beta", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "gamma", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, observations })
    const { container } = render(<LiveCapacityCard provider="" />)

    const initialOrder = readGridAuthIndexes(getSectionGrids(container)[0])
    expect(initialOrder).toEqual(["alpha", "beta", "gamma"])

    const chipGroup = screen.getByRole("group", { name: "Filter accounts by provider" })
    await user.click(within(chipGroup).getByRole("button", { name: /Claude/ }))
    expect(readGridAuthIndexes(getSectionGrids(container)[0])).toEqual(["beta"])

    await user.click(within(chipGroup).getByRole("button", { name: /^All/ }))
    expect(readGridAuthIndexes(getSectionGrids(container)[0])).toEqual(initialOrder)
  })

  it("asks for inline confirmation before disabling an account", async () => {
    const user = userEvent.setup()
    const identities = [identity({ id: 7, identity: "codex-auth", displayName: "Codex Auth" })]
    setupMock({ identities })
    render(<LiveCapacityCard provider="" />)
    mockSetIdentityDisabled.mutate.mockClear()
    mockToast.success.mockClear()

    await user.click(screen.getByRole("button", { name: "Disable Codex Auth" }))
    expect(mockSetIdentityDisabled.mutate).not.toHaveBeenCalled()

    await user.click(screen.getByRole("button", { name: "Confirm disabling Codex Auth" }))
    expect(mockSetIdentityDisabled.mutate).toHaveBeenCalledWith(
      { id: 7, disabled: true },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    )

    const [, options] = mockSetIdentityDisabled.mutate.mock.calls[0]
    options.onSuccess()
    expect(mockToast.success).toHaveBeenCalledWith("Account disabled")
    options.onError("unexpected response")
    expect(mockToast.error).toHaveBeenCalledWith("Failed to update account")
  })

  it("abandons the disable confirmation when the second click never comes", () => {
    vi.useFakeTimers()
    try {
      const identities = [identity({ id: 7, identity: "codex-auth", displayName: "Codex Auth" })]
      setupMock({ identities })
      render(<LiveCapacityCard provider="" />)
      mockSetIdentityDisabled.mutate.mockClear()

      fireEvent.click(screen.getByRole("button", { name: "Disable Codex Auth" }))
      expect(screen.getByRole("button", { name: "Confirm disabling Codex Auth" })).toBeInTheDocument()

      act(() => { vi.advanceTimersByTime(3_100) })
      expect(screen.queryByRole("button", { name: "Confirm disabling Codex Auth" })).not.toBeInTheDocument()
      expect(screen.getByRole("button", { name: "Disable Codex Auth" })).toBeInTheDocument()
      expect(mockSetIdentityDisabled.mutate).not.toHaveBeenCalled()
    } finally {
      vi.useRealTimers()
    }
  })

  it("enables a disabled account with a single click and no confirmation", async () => {
    const user = userEvent.setup()
    const identities = [identity({ id: 9, identity: "codex-auth", displayName: "Codex Auth", disabled: true })]
    setupMock({ identities })
    render(<LiveCapacityCard provider="" />)
    mockSetIdentityDisabled.mutate.mockClear()
    mockToast.success.mockClear()

    await user.click(screen.getByRole("button", { name: "Enable Codex Auth" }))
    expect(mockSetIdentityDisabled.mutate).toHaveBeenCalledWith(
      { id: 9, disabled: false },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    )

    const [, options] = mockSetIdentityDisabled.mutate.mock.calls[0]
    options.onSuccess()
    expect(mockToast.success).toHaveBeenCalledWith("Account enabled")
    options.onError(new Error("CPA accepted the change, but the resulting account state could not be confirmed. Run Trigger Sync to reload account status."))
    expect(mockToast.error).toHaveBeenCalledWith(
      "CPA accepted the change, but the resulting account state could not be confirmed. Run Trigger Sync to reload account status.",
    )
  })

  it("refreshes a disabled account without enabling it and retains its disabled presentation", async () => {
    const user = userEvent.setup()
    const identities = [identity({ identity: "codex-auth", displayName: "Codex Auth", disabled: true })]
    const observations: QuotaObservationsResponse = {
      items: [{ id: "codex-auth", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] }],
    }
    const mock = setupMock({ identities, observations })
    mockSetIdentityDisabled.mutate.mockClear()
    const { container } = render(<LiveCapacityCard provider="" />)

    expect(container.querySelector(".group.opacity-60")).not.toBeNull()
    // 状态 chip 不再重复渲染 Disabled，标题旁的琥珀徽章是唯一状态标识
    expect(screen.getAllByText("Disabled")).toHaveLength(1)
    const amberBadge = screen.getAllByText("Disabled").find((el) => el.className.includes("bg-amber-500/10"))
    expect(amberBadge).toBeDefined()
    expect(screen.queryByRole("group", { name: "Account availability" })).not.toBeInTheDocument()
    const refresh = screen.getByRole("button", { name: "Refresh Codex Auth" })
    expect(refresh).toBeEnabled()
    await user.click(refresh)
    expect(mock.refresh).toHaveBeenCalledWith("codex-auth")
    expect(mockSetIdentityDisabled.mutate).not.toHaveBeenCalled()
    expect(screen.getByRole("button", { name: "Enable Codex Auth" })).toBeInTheDocument()
  })

  it.each(["starting", "queued", "running"] as const)("shows %s quota refresh progress on a disabled account", (status) => {
    setupMock({
      identities: [identity({ disabled: true })],
      taskStates: { "codex-auth": { status, taskId: "task-1" } },
    })
    render(<LiveCapacityCard provider="" />)

    expect(screen.getByText("Disabled")).toBeInTheDocument()
    const refresh = screen.getByRole("button", { name: "Refresh Codex Auth" })
    expect(refresh).toBeDisabled()
    expect(refresh.querySelector(".animate-spin")).not.toBeNull()
  })

  it("shows a disabled account's quota refresh error with a retry action and retained reading", () => {
    setupMock({
      identities: [identity({ disabled: true })],
      observations: {
        items: [{ id: "codex-auth", observedAt: OBSERVED_AT, quota: [{ key: "primary", label: "5h", usedPercent: 10 }] }],
      },
      taskStates: { "codex-auth": { status: "failed", error: "HTTP 401" } },
    })
    render(<LiveCapacityCard provider="" />)

    expect(screen.getByText("Disabled")).toBeInTheDocument()
    expect(screen.getByTitle("Refresh failed: HTTP 401")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Refresh Codex Auth" })).toBeEnabled()
    expect(screen.getByText("10% used")).toBeInTheDocument()
  })

  it("sinks disabled accounts to the end of the regular section", () => {
    const identities = [
      identity({ identity: "beta-codex", displayName: "Beta", provider: "Codex", type: "codex" }),
      identity({ identity: "gamma-codex", displayName: "Gamma", provider: "Codex", type: "codex" }),
      identity({ identity: "alpha-codex", displayName: "Alpha", provider: "Codex", type: "codex", disabled: true }),
    ]
    setupMock({ identities })
    const { container } = render(<LiveCapacityCard provider="" />)

    expect(readGridAuthIndexes(getSectionGrids(container)[0])).toEqual(["beta-codex", "gamma-codex", "alpha-codex"])
  })
})
