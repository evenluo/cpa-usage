import { describe, expect, it } from "vitest"
import { buildAttemptPerformancePath } from "./useAttemptPerformance"

describe("buildAttemptPerformancePath", () => {
  it("uses the fixed 24-hour window and optional provider scope", () => {
    expect(buildAttemptPerformancePath("")).toBe("/usage/performance?range=24h")
    expect(buildAttemptPerformancePath("OpenAI Mirror")).toBe("/usage/performance?range=24h&provider=OpenAI+Mirror")
  })
})
