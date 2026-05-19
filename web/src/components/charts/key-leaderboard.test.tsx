import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import type { KeyAliasBreakdown } from "@/types/api"
import { KeyLeaderboard } from "./key-leaderboard"

function keyBreakdown(overrides: Partial<KeyAliasBreakdown> = {}): KeyAliasBreakdown {
  return {
    label: "Agent API Key",
    alias: "",
    traceability: "sk-a*******alue · OpenAI",
    identity: "sk-a*******alue",
    auth_type: 2,
    auth_type_name: "apikey",
    type: "",
    provider: "OpenAI",
    is_deleted: false,
    total_cost: 1.25,
    total_tokens: 1200,
    request_count: 3,
    success_count: 3,
    failure_count: 0,
    success_rate: 100,
    last_used_at: null,
    cost_available: true,
    cost_status: "available",
    trend: [],
    ...overrides,
  }
}

describe("KeyLeaderboard", () => {
  it("keeps the API-provided display label before falling back to the masked identity", () => {
    render(<KeyLeaderboard data={[keyBreakdown()]} />)

    expect(screen.getByText("Agent API Key")).toBeInTheDocument()
    expect(screen.queryByText("sk-a*******alue", { selector: "p.text-sm" })).not.toBeInTheDocument()
  })

  it("uses a saved alias as the primary display label", () => {
    render(<KeyLeaderboard data={[keyBreakdown({ alias: "Production Agent" })]} />)

    expect(screen.getByText("Production Agent")).toBeInTheDocument()
  })
})
