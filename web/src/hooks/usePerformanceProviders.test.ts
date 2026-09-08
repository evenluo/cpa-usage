import { describe, expect, it } from "vitest"
import { buildPerformanceProvidersPath } from "./usePerformanceProviders"

describe("performance provider catalog", () => {
  it("uses the fixed 24-hour narrow endpoint", () => {
    expect(buildPerformanceProvidersPath()).toBe("/usage/performance/providers?range=24h")
  })
})
