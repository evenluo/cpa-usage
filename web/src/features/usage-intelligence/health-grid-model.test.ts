import { describe, expect, it } from "vitest"
import type { ServiceHealthBlock } from "@/types/api"
import { cellColor, computeLayout } from "./health-grid-model"

function block(overrides: Partial<ServiceHealthBlock> = {}): ServiceHealthBlock {
  return {
    start_time: "2026-05-18T00:00:00Z",
    end_time: "2026-05-18T00:03:00Z",
    success: 3,
    failure: 0,
    rate: 1,
    ...overrides,
  }
}

describe("cellColor", () => {
  it("mutes blocks without a rate or without requests", () => {
    expect(cellColor(block({ rate: -1 }))).toBe("bg-muted/20")
    expect(cellColor(block({ success: 0, failure: 0, rate: 0 }))).toBe("bg-muted/20")
  })

  it("marks fully successful blocks as perfect", () => {
    expect(cellColor(block({ success: 5, failure: 0, rate: 1 }))).toBe("bg-emerald-500")
  })

  it("grades blocks with failures by success rate", () => {
    expect(cellColor(block({ success: 99, failure: 1, rate: 0.99 }))).toBe("bg-emerald-400")
    expect(cellColor(block({ success: 19, failure: 1, rate: 0.95 }))).toBe("bg-amber-400")
    expect(cellColor(block({ success: 9, failure: 1, rate: 0.9 }))).toBe("bg-red-400")
  })
})

describe("computeLayout", () => {
  it("returns a single column when there are no blocks", () => {
    expect(computeLayout(1200, 0)).toEqual({ columns: 1 })
  })

  it("targets six rows on wide containers", () => {
    expect(computeLayout(1200, 480)).toEqual({ columns: 80 })
  })

  it("packs more rows when the container is narrow", () => {
    expect(computeLayout(200, 480)).toEqual({ columns: 20 })
  })

  it("never exceeds the max columns the container width allows", () => {
    expect(computeLayout(60, 10)).toEqual({ columns: 1 })
  })
})
