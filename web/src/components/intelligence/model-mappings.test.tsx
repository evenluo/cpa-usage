import { cleanup, fireEvent, render, screen } from "@testing-library/react"
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
  it("starts as a compact 24-hour summary and expands into attempt-share mapping bars", () => {
    render(<ModelMappings data={fixture as UsageModelMappingDistribution} isLoading={false} error={null} onRetry={vi.fn()} />)

    const summary = screen.getByText("Observed model mappings").closest("summary")
    const details = summary?.closest("details")
    expect(details).not.toHaveAttribute("open")
    expect(summary?.querySelector("div")).toBeNull()
    expect(summary?.querySelector(":scope > h3")).toHaveTextContent("Observed model mappings")
    expect(summary).toHaveTextContent("4 mappings displayed")
    expect(summary).toHaveTextContent("83.3% alias coverage")
    expect(summary).toHaveTextContent("1 missing alias")
    expect(screen.getByText("Last 24h")).toHaveAttribute("title", expect.stringContaining("–"))

    fireEvent.click(summary!)

    expect(details).toHaveAttribute("open")
    expect(screen.queryByText("Observed alias to actual model")).not.toBeInTheDocument()
    expect(screen.getByText("Share of alias-bearing attempts")).toBeVisible()
    const mappingNote = screen.getByText(/Names are observed values/)
    expect(mappingNote).not.toBeVisible()
    fireEvent.click(screen.getByText("About these mappings"))
    expect(mappingNote).toBeVisible()
    expect(screen.getByRole("img", { name: "40.0% of attempts with an observed alias" })).toBeInTheDocument()
    expect(screen.getAllByText("route-a")).toHaveLength(2)
    expect(screen.getByText("provider-a")).toBeInTheDocument()
    expect(screen.getByText("Same observed name")).toBeInTheDocument()
    expect(screen.queryByText("Direct")).not.toBeInTheDocument()
    expect(screen.getByText("Observed cost · $1.00 · Partial")).toBeInTheDocument()
    expect(screen.getByText("Mean latency · 100 ms")).toHaveAttribute("title", "1 sample")
    expect(screen.getByRole("link", { name: "Inspect route-a to actual-a attempts" })).toHaveAttribute("href", expect.stringContaining("modelAlias=route-a"))
  })

  it("preserves unavailable values and non-link rows without inferring routing", () => {
    const missingModel = {
      ...fixture,
      other_attempts: 2,
      mappings: [{ ...fixture.mappings[0], model: "", provider: "" }],
    } as UsageModelMappingDistribution

    render(<ModelMappings data={missingModel} isLoading={false} error={null} onRetry={vi.fn()} />)

    expect(screen.getByText("Actual model unavailable")).toBeInTheDocument()
    expect(screen.getByText("Provider unavailable")).toBeInTheDocument()
    expect(screen.getByText("Other observed attempts · 2")).toBeInTheDocument()
    expect(screen.queryByRole("link")).not.toBeInTheDocument()
  })

  it("does not present a zero cost when alias coverage is absent", () => {
    render(<ModelMappings data={{ ...fixture, observed_alias_attempts: 0, missing_alias_attempts: 6, alias_coverage: 0, observed_total_cost: 0, mappings: [] } as UsageModelMappingDistribution} isLoading={false} error={null} onRetry={vi.fn()} />)
    expect(screen.getByText("Observed cost · No observed alias population")).toBeInTheDocument()
  })

  it("keeps a refresh failure visible when stale mapping data is available", () => {
    const onRetry = vi.fn()
    render(<ModelMappings data={fixture as UsageModelMappingDistribution} isLoading={false} error={new Error("refresh failed")} onRetry={onRetry} />)

    expect(screen.getByRole("alert")).toHaveTextContent("Latest refresh failed. Showing previously loaded data.")
    fireEvent.click(screen.getByRole("button", { name: "Retry refresh" }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it("keeps retry available when stale data contains no attempts", () => {
    const onRetry = vi.fn()
    const emptyData = { ...fixture, total_attempts: 0, observed_alias_attempts: 0, missing_alias_attempts: 0, mappings: [] } as UsageModelMappingDistribution
    render(<ModelMappings data={emptyData} isLoading={false} error={new Error("refresh failed")} onRetry={onRetry} />)

    expect(screen.getByText("No attempts in the last 24 hours")).toBeInTheDocument()
    expect(screen.getByRole("alert")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Retry refresh" }))
    expect(onRetry).toHaveBeenCalledOnce()
  })
})
