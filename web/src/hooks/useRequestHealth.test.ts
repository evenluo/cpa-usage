import { describe, expect, it } from "vitest"
import { buildRequestHealthPath } from "./useRequestHealth"

describe("useRequestHealth", () => {
  it("reads Request Health from the dedicated endpoint", () => {
    expect(buildRequestHealthPath("24h", "OpenAI")).toBe(
      "/usage/request-health?range=24h&provider=OpenAI",
    )
  })

  it("omits provider when all providers are selected", () => {
    expect(buildRequestHealthPath("24h", "")).toBe("/usage/request-health?range=24h")
  })
})
