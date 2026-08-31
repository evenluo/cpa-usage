import { describe, expect, it } from "vitest"
import { normalizeRequestsSearch } from "./requests"

describe("requests search validation", () => {
  it.each([
    [{}, ""],
    [{ provider: "" }, ""],
    [{ provider: "   " }, ""],
    [{ provider: 42 }, ""],
    [{ provider: ["claude"] }, ""],
  ])("normalizes missing and malformed provider state", (search, expected) => {
    expect(normalizeRequestsSearch(search)).toEqual({ provider: expected })
  })

  it("trims and preserves unknown non-empty providers", () => {
    expect(normalizeRequestsSearch({ provider: "  future-provider  " })).toEqual({
      provider: "future-provider",
    })
  })
})
