import { describe, expect, it } from "vitest"
import { buildAnalyticsCorePath, buildAnalyticsHeatmapPath } from "./useAnalytics"

describe("useAnalytics", () => {
  it("reads Usage Intelligence core analytics from the core endpoint", () => {
    expect(buildAnalyticsCorePath("7d", "hour", "OpenAI")).toBe(
      "/analytics/core?range=7d&granularity=hour&provider=OpenAI",
    )
  })

  it("reads Activity Heatmap from the dedicated heatmap endpoint", () => {
    expect(buildAnalyticsHeatmapPath("30d", "day", "OpenAI")).toBe(
      "/analytics/heatmap?range=30d&granularity=day&provider=OpenAI",
    )
  })

  it("omits provider when all providers are selected", () => {
    expect(buildAnalyticsCorePath("30d", "day", "")).toBe(
      "/analytics/core?range=30d&granularity=day",
    )
  })
})
