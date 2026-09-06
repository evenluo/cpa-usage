import { describe, expect, it } from "vitest"
import { buildFailureDistributionPath } from "./useFailureDistribution"

describe("failure distribution query", () => {
  it("uses the fixed window and preserves provider scope", () => {
    expect(buildFailureDistributionPath("")).toBe("/usage/failures?range=24h")
    expect(buildFailureDistributionPath("OpenAI Mirror")).toBe("/usage/failures?range=24h&provider=OpenAI+Mirror")
  })
})
