import { describe, expect, it } from "vitest"
import { buildModelMappingsPath, buildModelMappingsSummaryPath } from "./useModelMappings"

describe("buildModelMappingsPath", () => {
  it("uses the fixed diagnostic window and selected provider", () => {
    expect(buildModelMappingsPath("provider a")).toBe("/usage/model-mappings?range=24h&provider=provider+a")
    expect(buildModelMappingsSummaryPath("provider a")).toBe("/usage/model-mappings/summary?range=24h&provider=provider+a")
  })

  it("freezes detail rows to the summary snapshot", () => {
    expect(buildModelMappingsPath("provider a", "2026-09-08T12:00:00Z")).toBe(
      "/usage/model-mappings?range=24h&provider=provider+a&window_end=2026-09-08T12%3A00%3A00Z",
    )
  })
})
