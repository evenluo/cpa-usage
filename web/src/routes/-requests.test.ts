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
    expect(normalizeRequestsSearch(search)).toEqual({ provider: expected, model: "", modelAlias: "", account: "", endpoint: "", status: "", requestId: "", windowEnd: "", result: "" })
  })

  it("trims and preserves unknown non-empty providers", () => {
    expect(normalizeRequestsSearch({ provider: "  future-provider  " })).toEqual({
      provider: "future-provider",
      model: "",
      modelAlias: "",
      account: "",
      endpoint: "",
      status: "",
      requestId: "",
      windowEnd: "",
      result: "",
    })
  })

  it("keeps a trimmed model and only accepted attempt results", () => {
    expect(normalizeRequestsSearch({ model: "  gpt-5  ", result: "failed" })).toEqual({
      provider: "",
      model: "gpt-5",
      modelAlias: "",
      account: "",
      endpoint: "",
      status: "",
      requestId: "",
      windowEnd: "",
      result: "failed",
    })
    expect(normalizeRequestsSearch({ model: 42, result: "unknown" })).toEqual({
      provider: "",
      model: "",
      modelAlias: "",
      account: "",
      endpoint: "",
      status: "",
      requestId: "",
      windowEnd: "",
      result: "",
    })
  })

  it("preserves diagnostic values for server-owned validation", () => {
    expect(normalizeRequestsSearch({ account: " auth-1 ", endpoint: " /v1/messages ", status: " 4XX ", minLatencyMS: " 500 " })).toEqual({
      provider: "", model: "", modelAlias: "", account: "auth-1", endpoint: "/v1/messages", status: "4xx", requestId: "", minLatencyMS: "500", windowEnd: "", result: "",
    })
  })

  it("preserves the observed alias selection", () => {
    expect(normalizeRequestsSearch({ modelAlias: " route-a " }).modelAlias).toBe("route-a")
  })
})
