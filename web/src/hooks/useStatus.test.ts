import { describe, expect, it } from "vitest"
import { MANUAL_SYNC_READ_MODEL_QUERY_KEYS } from "./useStatus"

describe("manual sync invalidation", () => {
  it("refreshes usage, evidence, identity, and reference-data read models", () => {
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).toEqual([
      ["analytics"],
      ["usage"],
      ["events"],
      ["keys"],
      ["pricing"],
    ])
  })

  it("covers both identity lists through the keys prefix", () => {
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).toContainEqual(["keys"])
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).not.toContainEqual(["keys", "identities"])
    expect(MANUAL_SYNC_READ_MODEL_QUERY_KEYS).not.toContainEqual(["keys", "api-keys"])
  })
})
