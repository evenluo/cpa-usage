import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it } from "vitest"
import type { KeyAliasBreakdown } from "@/types/api"
import { KeyLeaderboard } from "./key-leaderboard"

afterEach(cleanup)

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
    canonical_valid_attempts: 1,
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

  it("ranks and computes cost share from complete local estimates only", () => {
    const { container } = render(<KeyLeaderboard data={[
      keyBreakdown({ identity: "partial", label: "Partial", total_cost: 100, cost_available: false, cost_status: "partial" }),
      keyBreakdown({ identity: "complete", label: "Complete", total_cost: 20 }),
    ]} />)

    const labels = Array.from(container.querySelectorAll("p.text-sm.font-medium")).map((node) => node.textContent)
    expect(labels).toEqual(["Complete", "Partial"])
    expect(screen.getByText("100.0% cost", { exact: false })).toBeInTheDocument()
    expect(screen.getByText("cost n/a", { exact: false, selector: "p" })).toBeInTheDocument()
    expect(screen.getByText("Cost n/a", { exact: true })).toBeInTheDocument()
  })
})
