import { cleanup, render } from "@testing-library/react"
import { bench, describe, expect, vi } from "vitest"
import type { LiveCapacityTaskState } from "@/hooks/useQuota"
import {
  buildLiveCapacityRows,
  type LiveCapacityRow,
} from "@/features/usage-intelligence/live-capacity"
import type { KeyIdentity, QuotaCacheResponse, QuotaRow } from "@/types/api"

const mockUseLiveCapacity = vi.fn()

vi.mock("@/hooks/useQuota", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/hooks/useQuota")>()
  return {
    ...original,
    useLiveCapacity: (...args: Parameters<typeof original.useLiveCapacity>) => mockUseLiveCapacity(...args),
  }
})

vi.mock("@/hooks/useFlipReorder", () => ({
  useFlipReorder: () => ({
    containerRef: { current: null },
    registerItem: () => () => {},
  }),
}))

import { LiveCapacityCard } from "./live-capacity-card"

interface LiveCapacityFixture {
  accountCount: number
  quotaRowsPerAccount: number
  identities: KeyIdentity[]
  cachedQuota: QuotaCacheResponse
  taskStates: Record<string, LiveCapacityTaskState>
  refresh: (authIndex?: string) => void
  refreshLimit: number
  isLoading: boolean
  isRefreshing: boolean
  error: null
}

function quotaRows(count: number): QuotaRow[] {
  return Array.from({ length: count }, (_, index) => {
    if (index === 0) {
      return { key: "rate_limit.primary_window", label: "5h", usedPercent: 20, window: { seconds: 18_000 } }
    }
    if (index === 1) {
      return { key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 40, window: { seconds: 604_800 } }
    }
    return {
      key: `model-${index}`,
      label: `Model capacity ${index}`,
      remainingFraction: ((index * 7) % 100) / 100,
    }
  })
}

function createFixture(accountCount: number, quotaRowsPerAccount: number): LiveCapacityFixture {
  const rows = quotaRows(quotaRowsPerAccount)
  const identities = Array.from({ length: accountCount }, (_, index): KeyIdentity => ({
    id: index + 1,
    name: `capacity-account-${index + 1}`,
    displayName: `Capacity account ${index + 1}`,
    alias: "",
    auth_type: 1,
    auth_type_name: "oauth",
    identity: `capacity-account-${index + 1}`,
    type: "codex",
    provider: "Codex",
    plan_type: index % 10 === 0 ? "pro" : "team",
    total_tokens: 0,
    total_cost: 0,
    cost_available: false,
    last_used_at: null,
  }))

  return {
    accountCount,
    quotaRowsPerAccount,
    identities,
    cachedQuota: {
      items: identities.map((identity) => ({ id: identity.identity, quota: rows })),
    },
    taskStates: {},
    refresh: () => {},
    refreshLimit: 20,
    isLoading: false,
    isRefreshing: false,
    error: null,
  }
}

const capacity100x8 = createFixture(100, 8)
const capacity100x64 = createFixture(100, 64)
const capacity20x64 = createFixture(20, 64)
let liveCapacityRowsSink: LiveCapacityRow[] = []

function capacityRows(fixture: LiveCapacityFixture): LiveCapacityRow[] {
  return buildLiveCapacityRows({
    identities: fixture.identities,
    cachedQuota: fixture.cachedQuota,
    taskStates: fixture.taskStates,
  })
}

function renderAndCleanup(fixture: LiveCapacityFixture): void {
  mockUseLiveCapacity.mockReturnValue(fixture)
  try {
    render(<LiveCapacityCard provider="" />)
  } finally {
    cleanup()
    document.body.replaceChildren()
  }
}

function assertRenderedMeterCount(fixture: LiveCapacityFixture): void {
  mockUseLiveCapacity.mockReturnValue(fixture)
  try {
    const view = render(<LiveCapacityCard provider="" />)
    expect(view.container.querySelectorAll("[aria-label*=': ']")).toHaveLength(
      fixture.accountCount * fixture.quotaRowsPerAccount,
    )
  } finally {
    cleanup()
    document.body.replaceChildren()
  }
}

for (const fixture of [capacity100x8, capacity100x64, capacity20x64]) {
  assertRenderedMeterCount(fixture)
}

describe("Live Capacity scale", () => {
  bench("build rows: 100 accounts x 8 quota rows", () => {
    liveCapacityRowsSink = capacityRows(capacity100x8)
    void liveCapacityRowsSink.length
  })

  bench("build rows: 100 accounts x 64 quota rows", () => {
    liveCapacityRowsSink = capacityRows(capacity100x64)
    void liveCapacityRowsSink.length
  })

  bench("render+cleanup: 100 accounts x 8 quota rows", () => {
    renderAndCleanup(capacity100x8)
  })

  bench("render+cleanup: 20 accounts x 64 quota rows", () => {
    renderAndCleanup(capacity20x64)
  })
})
