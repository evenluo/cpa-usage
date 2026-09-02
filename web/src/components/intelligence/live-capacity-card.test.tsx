import { act } from "react"
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { KeyIdentity, QuotaCacheResponse } from "@/types/api"
import type { LiveCapacityTaskState } from "@/hooks/useQuota"

// Mock the useLiveCapacity hook
const mockUseLiveCapacity = vi.fn()
vi.mock("@/hooks/useQuota", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/hooks/useQuota")>()
  return {
    ...original,
    useLiveCapacity: (...args: Parameters<typeof original.useLiveCapacity>) => mockUseLiveCapacity(...args),
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
    total_cost: 0,
    cost_available: false,
    last_used_at: null,
    ...overrides,
  }
}

interface LiveCapacityReturn {
  identities: KeyIdentity[]
  cachedQuota: QuotaCacheResponse | undefined
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
    cachedQuota: undefined,
    taskStates: {},
    refresh: vi.fn(),
    refreshLimit: 20,
    isLoading: false,
    isRefreshing: false,
    error: null,
  }
  const merged = { ...defaults, ...props }
  mockUseLiveCapacity.mockReturnValue(merged)
  return merged
}

/** Select the section-level grids (not the MetricMeter inner grids). */
function getSectionGrids(container: Element): Element[] {
  return Array.from(container.querySelectorAll(".grid.grid-cols-1"))
}

/** Read authIndex list from a grid element's children. */
function readGridAuthIndexes(grid: Element): string[] {
  return Array.from(grid.children).map((child) => {
    const authIndexEl = child.querySelector("p.truncate.text-xs")
    return authIndexEl?.textContent ?? ""
  })
}

describe("LiveCapacityCard", () => {
  afterEach(cleanup)

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

  it("renders tiles for each identity", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })
    render(<LiveCapacityCard provider="" />)

    expect(screen.getByText("Codex Pro")).toBeInTheDocument()
    expect(screen.getByText("Alpha Codex")).toBeInTheDocument()
  })

  it("visualizes probe freshness, subscription window, and additional quota rows", () => {
    const identities = [identity({
      identity: "codex-pro",
      displayName: "Codex Pro",
      provider: "Codex",
      type: "codex",
      active_start: "2026-08-01T00:00:00Z",
      active_until: "2026-09-01T00:00:00Z",
    })]
    const cachedQuota: QuotaCacheResponse = {
      items: [{
        id: "codex-pro",
        cachedAt: "2026-08-31T01:00:00Z",
        expiresAt: "2026-08-31T01:05:00Z",
        quota: [
          { key: "primary", label: "5h", usedPercent: 10 },
          { key: "secondary", label: "Weekly", usedPercent: 20 },
          { key: "review", label: "Code review", remaining: 15 },
        ],
      }],
    }
    setupMock({ identities, cachedQuota })
    render(<LiveCapacityCard provider="" />)

    expect(screen.getByText("Code review")).toBeInTheDocument()
    const timing = screen.getByRole("group", { name: "Account and cache timing" })
    expect(within(timing).getByText("Observed")).toBeInTheDocument()
    expect(within(timing).getByText("Cache expires")).toBeInTheDocument()
    // Past subscription starts are hidden; only the end date remains.
    expect(within(timing).getByText("Active until")).toBeInTheDocument()
    expect(within(timing).queryByText("Starts")).not.toBeInTheDocument()
    expect(within(timing).queryByText("Account active")).not.toBeInTheDocument()
    expect(timing.querySelectorAll("time")).toHaveLength(3)
    expect(timing.querySelector("time[datetime='2026-08-31T01:00:00Z']")).toBeInTheDocument()
    expect(timing.querySelector("time[datetime='2026-08-31T01:05:00Z']")).toBeInTheDocument()
    expect(timing.querySelector("time[datetime='2026-09-01T00:00:00Z']")).toBeInTheDocument()
  })

  it("renders a single cache-expiry endpoint without a connector when observedAt is missing", () => {
    const identities = [identity({
      identity: "codex-pro",
      displayName: "Codex Pro",
      provider: "Codex",
      type: "codex",
    })]
    const cachedQuota: QuotaCacheResponse = {
      items: [{
        id: "codex-pro",
        expiresAt: "2026-08-31T01:05:00Z",
        quota: [{ key: "primary", label: "5h", usedPercent: 10 }],
      }],
    }
    setupMock({ identities, cachedQuota })
    const { container } = render(<LiveCapacityCard provider="" />)

    const timing = within(container).getByRole("group", { name: "Account and cache timing" })
    expect(within(timing).getByText("Cache expires")).toBeInTheDocument()
    expect(within(timing).queryByText("Observed")).not.toBeInTheDocument()
    expect(timing.querySelectorAll("time")).toHaveLength(1)
    expect(timing.querySelector("time[datetime='2026-08-31T01:05:00Z']")).toBeInTheDocument()
    expect(timing.querySelector("svg.lucide-arrow-right")).not.toBeInTheDocument()
  })

  it("renders a single account-active endpoint without a connector when active start is missing", () => {
    const identities = [identity({
      identity: "codex-pro",
      displayName: "Codex Pro",
      provider: "Codex",
      type: "codex",
      active_until: "2026-09-01T00:00:00Z",
    })]
    const cachedQuota: QuotaCacheResponse = {
      items: [{
        id: "codex-pro",
        quota: [{ key: "primary", label: "5h", usedPercent: 10 }],
      }],
    }
    setupMock({ identities, cachedQuota })
    const { container } = render(<LiveCapacityCard provider="" />)

    const timing = within(container).getByRole("group", { name: "Account and cache timing" })
    expect(within(timing).getByText("Active until")).toBeInTheDocument()
    expect(within(timing).queryByText("Account active")).not.toBeInTheDocument()
    expect(within(timing).queryByText("Ends")).not.toBeInTheDocument()
    expect(within(timing).queryByText("Starts")).not.toBeInTheDocument()
    expect(timing.querySelectorAll("time")).toHaveLength(1)
    expect(timing.querySelector("time[datetime='2026-09-01T00:00:00Z']")).toBeInTheDocument()
    expect(timing.querySelector("svg.lucide-arrow-right")).not.toBeInTheDocument()
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
    const cachedQuota: QuotaCacheResponse = {
      items: [{
        id: "codex-pro",
        quota: [{ key: "primary", label: "5h", usedPercent: 10 }],
      }],
    }
    setupMock({ identities, cachedQuota })
    const { container } = render(<LiveCapacityCard provider="" />)

    const timing = within(container).getByRole("group", { name: "Account and cache timing" })
    expect(within(timing).getByText("Account active")).toBeInTheDocument()
    expect(within(timing).getByText("Starts")).toBeInTheDocument()
    expect(within(timing).getByText("Ends")).toBeInTheDocument()
    expect(timing.querySelector(`time[datetime='${futureStart}']`)).toBeInTheDocument()
    expect(timing.querySelector(`time[datetime='${futureUntil}']`)).toBeInTheDocument()
    expect(timing.querySelector("svg.lucide-arrow-right")).toBeInTheDocument()
  })

  it("separates priority accounts from regular accounts with a divider", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })
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
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "plain-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "team-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })
    const { container } = render(<LiveCapacityCard provider="" />)

    // Only one grid section (regular only)
    const grids = getSectionGrids(container)
    expect(grids).toHaveLength(1)

    // No divider
    expect(container.querySelector("[role='separator']")).not.toBeInTheDocument()
  })

  it("moves account to priority section when plan upgrades via taskState", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const initialCache: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota: initialCache })
    const { container, rerender } = render(<LiveCapacityCard provider="" />)

    // Initially: 1 priority (codex-pro), 1 regular (plain-codex)
    let grids = getSectionGrids(container)
    expect(readGridAuthIndexes(grids[0])).toEqual(["codex-pro"])
    expect(readGridAuthIndexes(grids[1])).toEqual(["plain-codex"])

    // plain-codex refreshes and upgrades to pro
    setupMock({
      identities,
      cachedQuota: initialCache,
      taskStates: {
        "plain-codex": {
          status: "completed",
          taskId: "task-1",
          quota: {
            id: "plain-codex",
            quota: [{ key: "quota", label: "5h", usedPercent: 20, planType: "pro" }],
          },
        },
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

  it("preserves regular section order when taskStates change", () => {
    const identities = [
      identity({ identity: "alpha-codex", displayName: "Alpha", provider: "Codex", type: "codex" }),
      identity({ identity: "beta-codex", displayName: "Beta", provider: "Codex", type: "codex" }),
    ]
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "alpha-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "beta-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })
    const { container, rerender } = render(<LiveCapacityCard provider="" />)

    const grids = getSectionGrids(container)
    const initialOrder = readGridAuthIndexes(grids[0])

    // Refresh beta-codex (no plan change)
    setupMock({
      identities,
      cachedQuota,
      taskStates: {
        "beta-codex": {
          status: "completed",
          taskId: "task-1",
          quota: {
            id: "beta-codex",
            quota: [{ key: "quota", label: "5h", usedPercent: 50, planType: "team" }],
          },
        },
      },
    })
    act(() => { rerender(<LiveCapacityCard provider="" />) })

    const gridsAfter = getSectionGrids(container)
    expect(readGridAuthIndexes(gridsAfter[0])).toEqual(initialOrder)
  })

  it("filters tiles via provider chips and restores the full list on All", async () => {
    const user = userEvent.setup()
    const identities = [
      identity({ identity: "codex-a", displayName: "Codex A", provider: "Codex", type: "codex" }),
      identity({ identity: "codex-b", displayName: "Codex B", provider: "Codex", type: "codex" }),
      identity({ identity: "claude-a", displayName: "Claude A", provider: "Claude", type: "claude" }),
    ]
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "codex-a", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "codex-b", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "claude-a", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })
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
    expect(screen.getByText("Claude A")).toBeInTheDocument()
    expect(claudeChip).toHaveAttribute("aria-pressed", "true")

    await user.click(allChip)
    expect(screen.getByText("Codex A")).toBeInTheDocument()
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
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "alpha", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "beta", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "gamma", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })
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
    options.onError()
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
  })

  it("renders a disabled account dimmed with an amber badge, a muted notice, and no refresh action", () => {
    const identities = [identity({ identity: "codex-auth", displayName: "Codex Auth", disabled: true })]
    const cachedQuota: QuotaCacheResponse = {
      items: [{ id: "codex-auth", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] }],
    }
    setupMock({ identities, cachedQuota })
    const { container } = render(<LiveCapacityCard provider="" />)

    expect(container.querySelector(".group.opacity-60")).not.toBeNull()
    const amberBadge = screen.getAllByText("Disabled").find((el) => el.className.includes("bg-amber-500/10"))
    expect(amberBadge).toBeDefined()
    expect(screen.getByText("Disabled in CPA — not routing requests")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Refresh Codex Auth" })).not.toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Enable Codex Auth" })).toBeInTheDocument()
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
