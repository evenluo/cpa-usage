import { describe, expect, it } from "vitest"
import { buildUsageIntelligenceLoadPlan } from "./load-plan"

describe("Usage Intelligence load plan", () => {
  it("separates selected-window analytics from fixed-window readings", () => {
    const plan = buildUsageIntelligenceLoadPlan({
      range: "30d",
      granularity: "day",
      provider: "OpenAI",
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
      pageSize: 10,
      provider: "OpenAI",
    })
    expect(plan.fixedWindow.liveCapacity).toEqual({
      provider: "OpenAI",
    })
  })
})
