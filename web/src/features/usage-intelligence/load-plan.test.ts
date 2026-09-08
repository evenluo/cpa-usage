import { describe, expect, it } from "vitest"
import { buildUsageIntelligenceLoadPlan } from "./load-plan"

describe("Usage Intelligence load plan", () => {
  it("separates selected-window analytics from fixed-window readings", () => {
    const plan = buildUsageIntelligenceLoadPlan({
      range: "30d",
      granularity: "day",
      provider: "OpenAI",
      attemptPerformanceProvider: "Anthropic",
    })

    expect(plan.selectedWindow.analytics).toEqual({
      range: "30d",
      granularity: "day",
      provider: "OpenAI",
    })
    expect(plan.fixedWindow.heatmap).toEqual({
      range: "30d",
      granularity: "day",
      provider: "OpenAI",
    })
    expect(plan.fixedWindow.requestHealth).toEqual({
      range: "24h",
      provider: "OpenAI",
    })
    expect(plan.fixedWindow.requestEvidence).toEqual({
      range: "24h",
      pageSize: 1,
      provider: "OpenAI",
    })
    expect(plan.fixedWindow.failureDistribution).toEqual({ range: "24h", provider: "OpenAI" })
    expect(plan.fixedWindow.modelMappings).toEqual({ range: "24h", provider: "OpenAI" })
    expect(plan.fixedWindow.attemptPerformance).toEqual({ range: "24h", provider: "Anthropic" })
    expect(plan.fixedWindow.liveCapacity).toEqual({
      provider: "OpenAI",
    })
  })
})
