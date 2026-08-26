import { describe, expect, it } from "vitest"
import type { TrendPoint } from "@/types/api"
import { cellColor, groupByDate } from "./health-timeline-model"

function trendPoint(overrides: Partial<TrendPoint> = {}): TrendPoint {
  return {
    label: "2026-05-09 22:00 +0800",
    total_cost: 1.25,
    total_tokens: 1200,
    input_tokens: 700,
    output_tokens: 400,
    reasoning_tokens: 80,
    cached_tokens: 20,
    request_count: 10,
    success_count: 10,
    failure_count: 0,
    cost_available: true,
    cost_status: "available",
    ...overrides,
  }
}

describe("groupByDate", () => {
  it("returns an empty map for an empty series", () => {
    expect(groupByDate([])).toEqual(new Map())
  })

  it("parses a timezone-suffixed hourly label into date, hour, and a short English label", () => {
    const groups = groupByDate([
      trendPoint({ label: "2026-05-09 22:00 +0800", request_count: 4, failure_count: 1 }),
    ])

    expect([...groups.keys()]).toEqual(["2026-05-09"])
    expect(groups.get("2026-05-09")).toEqual([
      {
        date: "2026-05-09",
        label: new Date("2026-05-09T00:00:00").toLocaleDateString("en", { month: "short", day: "numeric" }),
        hour: 22,
        success: 3,
        failure: 1,
        rate: 75,
      },
    ])
  })

  it("groups multiple days and sorts hours within each day", () => {
    const groups = groupByDate([
      trendPoint({ label: "2026-05-10 09:00 +0800", request_count: 2 }),
      trendPoint({ label: "2026-05-09 23:00 +0800", request_count: 1 }),
      trendPoint({ label: "2026-05-09 07:00 +0800", request_count: 5 }),
    ])

    expect([...groups.keys()]).toEqual(["2026-05-10", "2026-05-09"])
    expect(groups.get("2026-05-09")?.map((block) => block.hour)).toEqual([7, 23])
    expect(groups.get("2026-05-10")?.map((block) => block.hour)).toEqual([9])
  })

  it("keeps date-only labels and skips non-date labels", () => {
    const groups = groupByDate([
      trendPoint({ label: "2026-05-09", request_count: 8 }),
      trendPoint({ label: "n/a", request_count: 3 }),
      trendPoint({ label: "2026-05-09 22:00 +0800", request_count: 2 }),
    ])

    expect([...groups.keys()]).toEqual(["2026-05-09"])
    expect(groups.get("2026-05-09")).toEqual([
      expect.objectContaining({ date: "2026-05-09", hour: 0, success: 8, failure: 0 }),
      expect.objectContaining({ date: "2026-05-09", hour: 22, success: 2, failure: 0 }),
    ])
  })

  it("groups a day-bucket series by date into renderable blocks", () => {
    const groups = groupByDate([
      trendPoint({ label: "2026-05-11", request_count: 10, failure_count: 1 }),
      trendPoint({ label: "2026-05-12", request_count: 4, failure_count: 0 }),
      trendPoint({ label: "2026-05-11", request_count: 2, failure_count: 2 }),
    ])

    const shortLabel = (date: string) =>
      new Date(date + "T00:00:00").toLocaleDateString("en", { month: "short", day: "numeric" })

    expect([...groups.keys()]).toEqual(["2026-05-11", "2026-05-12"])
    expect(groups.get("2026-05-11")).toEqual([
      {
        date: "2026-05-11",
        label: shortLabel("2026-05-11"),
        hour: 0,
        success: 9,
        failure: 1,
        rate: 90,
      },
      {
        date: "2026-05-11",
        label: shortLabel("2026-05-11"),
        hour: 0,
        success: 0,
        failure: 2,
        rate: 0,
      },
    ])
    expect(groups.get("2026-05-12")).toEqual([
      {
        date: "2026-05-12",
        label: shortLabel("2026-05-12"),
        hour: 0,
        success: 4,
        failure: 0,
        rate: 100,
      },
    ])
  })

  it("clamps success at zero and reports a 0 rate when there are no requests", () => {
    const groups = groupByDate([
      trendPoint({ label: "2026-05-09 01:00 +0800", request_count: 2, failure_count: 5 }),
      trendPoint({ label: "2026-05-09 02:00 +0800", request_count: 0, failure_count: 0 }),
    ])

    expect(groups.get("2026-05-09")).toEqual([
      expect.objectContaining({ hour: 1, success: 0, failure: 5, rate: 0 }),
      expect.objectContaining({ hour: 2, success: 0, failure: 0, rate: 0 }),
    ])
  })

  it("rounds the success rate to one decimal place", () => {
    const groups = groupByDate([
      trendPoint({ label: "2026-05-09 03:00 +0800", request_count: 3, failure_count: 1 }),
    ])

    expect(groups.get("2026-05-09")?.[0].rate).toBe(66.7)
  })

  it("keeps duplicate hours instead of collapsing them", () => {
    const groups = groupByDate([
      trendPoint({ label: "2026-05-09 04:00 +0800", request_count: 1, failure_count: 0 }),
      trendPoint({ label: "2026-05-09 04:00 +0800", request_count: 2, failure_count: 1 }),
    ])

    expect(groups.get("2026-05-09")).toHaveLength(2)
    expect(groups.get("2026-05-09")?.map((block) => block.success)).toEqual([1, 1])
  })
})

describe("cellColor", () => {
  it("marks a block with no failures as perfect regardless of the numeric rate", () => {
    expect(cellColor(0, false)).toBe("bg-emerald-500")
    expect(cellColor(50, false)).toBe("bg-emerald-500")
  })

  it("grades failing blocks on the 99 / 95 thresholds", () => {
    expect(cellColor(99, true)).toBe("bg-emerald-400")
    expect(cellColor(98.9, true)).toBe("bg-amber-400")
    expect(cellColor(95, true)).toBe("bg-amber-400")
    expect(cellColor(94.9, true)).toBe("bg-red-400")
  })
})
