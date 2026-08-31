import { describe, expect, it } from "vitest"
import type { TrendPoint } from "@/types/api"
import {
  buildTrendSeriesConfig,
  buildTrendTickFormatter,
  mapTrendChartRows,
} from "./trend-model"

function hourlyLabels(day: string, hours: number[]): string[] {
  return hours.map((hour) => `${day} ${String(hour).padStart(2, "0")}:00 +0800`)
}

function trendPoint(overrides: Partial<TrendPoint> = {}): TrendPoint {
  return {
    label: "2026-05-09 22:00 +0800",
    total_cost: 1.25,
    total_tokens: 1200,
    input_tokens: 700,
    output_tokens: 400,
    reasoning_tokens: 80,
    cached_tokens: 20,
    request_count: 3,
    success_count: 3,
    failure_count: 0,
    cost_available: true,
    cost_status: "available",
    ...overrides,
  }
}

describe("buildTrendTickFormatter", () => {
  it("formats hour-only ticks for short hourly windows with timezone-suffixed labels", () => {
    const format = buildTrendTickFormatter(hourlyLabels("2026-05-09", [20, 21, 22]), "hour")

    expect(format("2026-05-09 22:00 +0800")).toBe("22:00")
    expect(format("2026-05-09 09:00 +0800")).toBe("09:00")
  })

  it("keeps hour-only ticks when a 24h window crosses midnight", () => {
    const labels = [...hourlyLabels("2026-05-11", [22, 23]), ...hourlyLabels("2026-05-12", [0, 1])]
    const format = buildTrendTickFormatter(labels, "hour")

    expect(format("2026-05-12 00:00 +0800")).toBe("00:00")
    expect(format("2026-05-11 23:00 +0800")).toBe("23:00")
  })

  it("shows dates only on day boundaries for multi-day hourly windows (7d)", () => {
    const labels = [
      ...hourlyLabels("2026-05-11", [12, 13]),
      ...hourlyLabels("2026-05-12", [0, 1]),
      ...hourlyLabels("2026-05-13", [0, 1]),
      ...hourlyLabels("2026-05-14", [0, 1]),
    ]
    const format = buildTrendTickFormatter(labels, "hour")

    expect(format("2026-05-12 00:00 +0800")).toBe("05/12")
    expect(format("2026-05-12 01:00 +0800")).toBe("")
    expect(format("2026-05-13 00:00 +0800")).toBe("05/13")
  })

  it("formats day-granularity labels as short month and day", () => {
    const format = buildTrendTickFormatter(["2026-05-09", "2026-05-10"], "day")

    expect(format("2026-05-09")).toBe("May 9")
    expect(format("2026-05-10")).toBe("May 10")
  })

  it("formats timezone-suffixed labels under day granularity by their date part", () => {
    const format = buildTrendTickFormatter(["2026-05-09"], "day")

    expect(format("2026-05-09 22:00 +0800")).toBe("May 9")
  })

  it("returns unparseable labels unchanged in every mode", () => {
    const dayFormat = buildTrendTickFormatter(["n/a"], "day")
    const hourFormat = buildTrendTickFormatter(["n/a"], "hour")

    expect(dayFormat("n/a")).toBe("n/a")
    expect(hourFormat("n/a")).toBe("n/a")
  })

  it("returns date-only labels unchanged in hour mode", () => {
    const format = buildTrendTickFormatter(hourlyLabels("2026-05-09", [10]), "hour")

    expect(format("2026-05-09")).toBe("2026-05-09")
  })
})

describe("buildTrendSeriesConfig", () => {
  it("maps cost-token mode to the cost area with a tokens overlay", () => {
    const config = buildTrendSeriesConfig("cost-token")

    expect(config.primaryKey).toBe("cost")
    expect(config.primaryName).toBe("Cost")
    expect(config.gradientId).toBe("costGradient")
    expect(config.overlayKey).toBe("tokens")
    expect(config.overlayName).toBe("Tokens")
  })

  it("swaps the overlay to requests in tokens mode", () => {
    const config = buildTrendSeriesConfig("tokens")

    expect(config.primaryKey).toBe("tokens")
    expect(config.overlayKey).toBe("requests")
    expect(config.overlayName).toBe("Attempts")
    expect(config.tokenSeries.map((series) => series.key)).toEqual([
      "inputTokens",
      "outputTokens",
      "reasoningTokens",
      "cachedTokens",
    ])
  })

  it("maps requests-token mode to the requests area", () => {
    const config = buildTrendSeriesConfig("requests-token")

    expect(config.primaryKey).toBe("requests")
    expect(config.primaryName).toBe("Attempts")
    expect(config.gradientId).toBe("requestGradient")
  })
})

describe("mapTrendChartRows", () => {
  it("maps trend points to chart rows", () => {
    const [row] = mapTrendChartRows([trendPoint()])

    expect(row).toEqual({
      label: "2026-05-09 22:00 +0800",
      cost: 1.25,
      requests: 3,
      tokens: 1200,
      inputTokens: 700,
      outputTokens: 400,
      reasoningTokens: 80,
      cachedTokens: 20,
      costStatus: "available",
    })
  })

  it("nulls out cost when the cost status is unavailable", () => {
    const [row] = mapTrendChartRows([
      trendPoint({ cost_available: false, cost_status: "unavailable" }),
    ])

    expect(row.cost).toBeNull()
    expect(row.costStatus).toBe("unavailable")
  })
})
