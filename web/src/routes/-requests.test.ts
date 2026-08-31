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
    expect(normalizeRequestsSearch(search)).toEqual({ provider: expected, model: "", result: "" })
  })

  it("trims and preserves unknown non-empty providers", () => {
    expect(normalizeRequestsSearch({ provider: "  future-provider  " })).toEqual({
      provider: "future-provider",
      model: "",
      result: "",
    })
  })

  it("keeps a trimmed model and only accepted attempt results", () => {
    expect(normalizeRequestsSearch({ model: "  gpt-5  ", result: "failed" })).toEqual({
      provider: "",
      model: "gpt-5",
      result: "failed",
    })
    expect(normalizeRequestsSearch({ model: 42, result: "unknown" })).toEqual({
      provider: "",
      model: "",
      result: "",
    })
  })
})
