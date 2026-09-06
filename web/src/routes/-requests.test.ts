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
    expect(normalizeRequestsSearch(search)).toEqual({ provider: expected, model: "", account: "", endpoint: "", status: "", windowEnd: "", result: "" })
  })

  it("trims and preserves unknown non-empty providers", () => {
    expect(normalizeRequestsSearch({ provider: "  future-provider  " })).toEqual({
      provider: "future-provider",
      model: "",
      account: "",
      endpoint: "",
      status: "",
      windowEnd: "",
      result: "",
    })
  })

  it("keeps a trimmed model and only accepted attempt results", () => {
    expect(normalizeRequestsSearch({ model: "  gpt-5  ", result: "failed" })).toEqual({
      provider: "",
      model: "gpt-5",
      account: "",
      endpoint: "",
      status: "",
      windowEnd: "",
      result: "failed",
    })
    expect(normalizeRequestsSearch({ model: 42, result: "unknown" })).toEqual({
      provider: "",
      model: "",
      account: "",
      endpoint: "",
      status: "",
      windowEnd: "",
      result: "",
    })
  })

  it("preserves diagnostic values for server-owned validation", () => {
    expect(normalizeRequestsSearch({ account: " auth-1 ", endpoint: " /v1/messages ", status: " 4XX " })).toEqual({
      provider: "", model: "", account: "auth-1", endpoint: "/v1/messages", status: "4xx", windowEnd: "", result: "",
    })
  })
})
