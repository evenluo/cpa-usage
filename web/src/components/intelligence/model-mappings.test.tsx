import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import fixture from "@/test/contracts/usage_model_mappings.json"
import type { UsageModelMappingDistribution } from "@/types/api"
import { ModelMappings } from "./model-mappings"

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, search, ...props }: { children: React.ReactNode; search: Record<string, string> }) => (
    <a href={`?${new URLSearchParams(search).toString()}`} {...props}>{children}</a>
  ),
}))

afterEach(cleanup)

describe("ModelMappings", () => {
  it("shows split observed mappings, coverage, semantics, cost states, and exact evidence selection", () => {
    render(<ModelMappings data={fixture as UsageModelMappingDistribution} isLoading={false} error={null} onRetry={vi.fn()} />)
    expect(screen.getByText("5").parentElement).toHaveTextContent("5 of 6 attempts have an observed alias (83.3%)")
    expect(screen.getByText("1 missing alias")).toBeInTheDocument()
    expect(screen.getAllByText("route-a")).toHaveLength(2)
    expect(screen.getByText("No distinct alias observed · Direct or canonicalized")).toBeInTheDocument()
    expect(screen.getByText("Provider unavailable")).toBeInTheDocument()
    expect(screen.getByText("Observed Cost: $1.00 · Partial")).toBeInTheDocument()
    expect(screen.getByText("100 ms · 1 sample")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Inspect route-a to actual-a attempts" })).toHaveAttribute("href", expect.stringContaining("modelAlias=route-a"))
    expect(screen.queryByText(/Requested model/i)).not.toBeInTheDocument()
  })

  it("does not present a zero Cost when alias coverage is absent", () => {
    render(<ModelMappings data={{ ...fixture, observed_alias_attempts: 0, missing_alias_attempts: 6, alias_coverage: 0, observed_total_cost: 0, mappings: [] } as UsageModelMappingDistribution} isLoading={false} error={null} onRetry={vi.fn()} />)
    expect(screen.getByText("Observed Cost: No observed alias population")).toBeInTheDocument()
  })
})
