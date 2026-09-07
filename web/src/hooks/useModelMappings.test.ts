import { describe, expect, it } from "vitest"
import { buildModelMappingsPath } from "./useModelMappings"

describe("buildModelMappingsPath", () => {
  it("uses the fixed diagnostic window and selected provider", () => {
    expect(buildModelMappingsPath("provider a")).toBe("/usage/model-mappings?range=24h&provider=provider+a")
  })
})
