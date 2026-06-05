import { act } from "react"
import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
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
  refresh: (authIndex?: string) => void
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
})
