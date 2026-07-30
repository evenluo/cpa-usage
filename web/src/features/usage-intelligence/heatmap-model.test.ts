import { describe, expect, it } from "vitest"
import type { HeatmapCell, HeatmapData } from "@/types/api"
import { computeLayout, flattenCells, splitRowLabel } from "./heatmap-model"

function heatmapCell(overrides: Partial<HeatmapCell> = {}): HeatmapCell {
  return {
    hour: 0,
    in_range: true,
    bucket_start: "2026-05-11T00:00:00Z",
    bucket_end: "2026-05-11T01:00:00Z",
    total_tokens: 100,
    total_cost: 0.5,
    request_count: 2,
    failure_count: 0,
    cost_available: true,
    cost_status: "available",
    ...overrides,
  }
}

function heatmapData(overrides: Partial<HeatmapData> = {}): HeatmapData {
  return {
    measure: "tokens",
    max_tokens: 100,
    max_cost: 1,
    max_requests: 2,
    max_failures: 0,
    rows: [],
    ...overrides,
  }
}

describe("flattenCells", () => {
  it("expands each row to a full 24-hour grid ordered by hour", () => {
    const data = heatmapData({
      rows: [
        { date: "2026-05-11", label: "05/11 Mon", cells: [heatmapCell({ hour: 3 }), heatmapCell({ hour: 0 })] },
        { date: "2026-05-12", label: "05/12 Tue", cells: [] },
      ],
    })

    const { cells, maxTokens } = flattenCells(data)

    expect(cells).toHaveLength(48)
    expect(cells.map((cell) => cell.hour)).toEqual([...Array(24).keys(), ...Array(24).keys()])
    expect(cells[0].cell?.hour).toBe(0)
    expect(cells[3].cell?.hour).toBe(3)
    expect(cells[1].cell).toBeNull()
    expect(cells[24].date).toBe("2026-05-12")
    expect(cells[24].dateLabel).toBe("05/12 Tue")
    expect(maxTokens).toBe(100)
  })

  it("floors maxTokens at 1 to avoid division by zero", () => {
    const { maxTokens } = flattenCells(heatmapData({ max_tokens: 0 }))

    expect(maxTokens).toBe(1)
  })
})

describe("splitRowLabel", () => {
  it("splits a combined date and weekday label", () => {
    expect(splitRowLabel("05/11 Mon", "2026-05-11")).toEqual({
      dateLabel: "05/11",
      weekdayLabel: "Mon",
    })
  })

  it("falls back to the row date when the label has no date fragment", () => {
    expect(splitRowLabel("Mon", "2026-05-11")).toEqual({
      dateLabel: "05/11",
      weekdayLabel: "Mon",
    })
  })

  it("returns an empty weekday when the label has no weekday fragment", () => {
    expect(splitRowLabel("2026-05-11", "2026-05-11")).toEqual({
      dateLabel: "05/11",
      weekdayLabel: "",
    })
  })

  it("keeps an unparseable date as-is", () => {
    expect(splitRowLabel("?", "week-20")).toEqual({
      dateLabel: "week-20",
      weekdayLabel: "",
    })
  })
})

describe("computeLayout", () => {
  it("fits three days per row on wide containers", () => {
    expect(computeLayout(1200)).toEqual({ daysPerRow: 3, cellSize: 14 })
  })

  it("drops to two days per row when three no longer fit", () => {
    expect(computeLayout(720)).toEqual({ daysPerRow: 2, cellSize: 12 })
  })

  it("drops to one day per row on narrow containers", () => {
    expect(computeLayout(400)).toEqual({ daysPerRow: 1, cellSize: 12 })
  })

  it("falls back to a single day at the default cell size below the minimum", () => {
    expect(computeLayout(100)).toEqual({ daysPerRow: 1, cellSize: 12 })
  })
})
