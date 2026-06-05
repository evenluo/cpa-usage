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

describe("LiveCapacityCard", () => {
  it("shows skeleton while loading", () => {
    setupMock({ isLoading: true })
    const { container } = render(<LiveCapacityCard provider="" />)
    // Skeleton elements have animate-pulse class
    expect(container.querySelectorAll(".animate-pulse")).toHaveLength(3)
  })

  it("shows empty state when no identities exist", () => {
    setupMock()
    render(<LiveCapacityCard provider="" />)
    expect(screen.getByText("No auth-file accounts")).toBeInTheDocument()
  })

  it("renders tiles for each identity in stable order", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })
    render(<LiveCapacityCard provider="" />)

    expect(screen.getByText("Codex Pro")).toBeInTheDocument()
    expect(screen.getByText("Alpha Codex")).toBeInTheDocument()
  })

  it("preserves tile order synchronously when taskStates change (no flicker)", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const cachedQuota: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota })

    const { container, rerender } = render(<LiveCapacityCard provider="" />)

    // Read tile order from grid children (each grid child is one tile wrapper)
    const getTileOrder = () => {
      const grid = container.querySelector(".grid")
      if (!grid) return []
      return Array.from(grid.children).map((child) => {
        // The authIndex is shown in a <p> with truncate text-xs
        const authIndexEl = child.querySelector("p.truncate.text-xs")
        return authIndexEl?.textContent ?? ""
      })
    }
    const initialOrder = getTileOrder()

    // Simulate taskState change (refresh completed for plain-codex with new plan)
    setupMock({
      identities,
      cachedQuota,
      taskStates: {
        "plain-codex": {
          status: "completed",
          taskId: "task-1",
          quota: {
            id: "plain-codex",
            quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 20, planType: "pro" }],
          },
        },
      },
    })

    // Rerender synchronously — order should not change mid-frame
    act(() => {
      rerender(<LiveCapacityCard provider="" />)
    })

    // The tile order should be identical — render-time setState prevents any flash
    expect(getTileOrder()).toEqual(initialOrder)
  })

  it("preserves tile order when cachedQuota updates after taskStates (two-phase refresh)", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const initialCache: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "team" }] },
      ],
    }
    setupMock({ identities, cachedQuota: initialCache })
    const { container, rerender } = render(<LiveCapacityCard provider="" />)

    const getTileOrder = () => {
      const grid = container.querySelector(".grid")
      if (!grid) return []
      return Array.from(grid.children).map((child) => {
        const authIndexEl = child.querySelector("p.truncate.text-xs")
        return authIndexEl?.textContent ?? ""
      })
    }
    const initialOrder = getTileOrder()

    // Phase 1: taskStates updates (completed with new plan)
    setupMock({
      identities,
      cachedQuota: initialCache,
      taskStates: {
        "plain-codex": {
          status: "completed",
          taskId: "task-1",
          quota: {
            id: "plain-codex",
            quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 20, planType: "pro" }],
          },
        },
      },
    })
    act(() => { rerender(<LiveCapacityCard provider="" />) })
    const afterPhase1 = getTileOrder()

    // Phase 2: cachedQuota updates (cache refetch completes)
    const refreshedCache: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "plain-codex", quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 20, planType: "pro" }] },
      ],
    }
    setupMock({
      identities,
      cachedQuota: refreshedCache,
      taskStates: {
        "plain-codex": {
          status: "completed",
          taskId: "task-1",
          quota: {
            id: "plain-codex",
            quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 20, planType: "pro" }],
          },
        },
      },
    })
    act(() => { rerender(<LiveCapacityCard provider="" />) })
    const afterPhase2 = getTileOrder()

    // Order should be stable throughout all phases
    expect(afterPhase1).toEqual(initialOrder)
    expect(afterPhase2).toEqual(initialOrder)
  })
})
