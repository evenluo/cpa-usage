import { describe, expect, it } from "vitest"
import { cn } from "./utils"

describe("cn", () => {
  it("keeps a card background underneath a gradient while applying class overrides", () => {
    expect(cn(
      "bg-card p-6 text-card-foreground",
      "bg-linear-to-br from-amber-50/90 via-amber-50/40 to-transparent p-4",
    )).toBe("bg-card text-card-foreground bg-linear-to-br from-amber-50/90 via-amber-50/40 to-transparent p-4")
  })
})
