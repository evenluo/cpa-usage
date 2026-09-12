import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import fixture from "@/test/contracts/usage_model_mappings.json"
import type { UsageModelMappingDistribution } from "@/types/api"
import type { UsageModelMappingSummary } from "@/features/usage-intelligence/model-mapping-summary"
import { ModelMappings } from "./model-mappings"

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, search, ...props }: { children: React.ReactNode; search: Record<string, string> }) => (
    <a href={`?${new URLSearchParams(search).toString()}`} {...props}>{children}</a>
  ),
}))

afterEach(cleanup)

const summaryFixture: UsageModelMappingSummary = {
  window_start: fixture.window_start,
  window_end: fixture.window_end,
  total_attempts: fixture.total_attempts,
  observed_alias_attempts: fixture.observed_alias_attempts,
  missing_alias_attempts: fixture.missing_alias_attempts,
  alias_coverage: fixture.alias_coverage,
  displayed_mappings: fixture.mappings.length,
}

function renderMappings(overrides: Partial<React.ComponentProps<typeof ModelMappings>> = {}) {
  const props: React.ComponentProps<typeof ModelMappings> = {
    summary: summaryFixture,
    data: fixture as UsageModelMappingDistribution,
    isSummaryLoading: false,
    summaryError: null,
    isDetailsLoading: false,
    detailsError: null,
    onExpandedChange: vi.fn(),
    onRetrySummary: vi.fn(),
    onRetryDetails: vi.fn(),
    ...overrides,
  }
  return { ...render(<ModelMappings {...props} />), props }
}

describe("ModelMappings", () => {
  it("starts as a compact 24-hour summary and expands into attempt-share mapping bars", async () => {
    const { props } = renderMappings()

    const summary = screen.getByText("Model mappings").closest("summary")
    const details = summary?.closest("details")
    expect(details).not.toHaveAttribute("open")
    expect(summary?.querySelector("div")).toBeNull()
    expect(summary?.querySelector(":scope > h3")).toHaveTextContent("Model mappings")
    expect(summary).toHaveTextContent("4 mappings")
    expect(summary).toHaveTextContent("83.3% have an alias")
    expect(summary).toHaveTextContent("1 without alias")
    expect(screen.getByText("Last 24h")).toHaveAttribute("title", expect.stringContaining("–"))

    fireEvent.click(summary!)

    expect(details).toHaveAttribute("open")
    await waitFor(() => expect(props.onExpandedChange).toHaveBeenCalledWith(true))
    expect(screen.queryByText("Observed alias to actual model")).not.toBeInTheDocument()
    expect(screen.getByText("Share of attempts with an alias")).toBeVisible()
    const mappingNote = screen.getByText(/Same name does not mean it was routed that way/)
    expect(mappingNote).not.toBeVisible()
    fireEvent.click(screen.getByText("About these mappings"))
    expect(mappingNote).toBeVisible()
    expect(screen.getByRole("img", { name: "40.0% of attempts with an observed alias" })).toBeInTheDocument()
    expect(screen.getAllByText("route-a")).toHaveLength(2)
    expect(screen.getByText("provider-a")).toBeInTheDocument()
    expect(screen.getByText("Same name")).toBeInTheDocument()
    expect(screen.queryByText("Direct")).not.toBeInTheDocument()
    expect(screen.getByText("Cost · $1.00 · Partial")).toBeInTheDocument()
    expect(screen.getByText("Mean latency · 100 ms")).toHaveAttribute("title", "1 sample")
    expect(screen.getByRole("link", { name: "Inspect route-a to actual-a attempts" })).toHaveAttribute("href", expect.stringContaining("modelAlias=route-a"))
  })

  it("preserves unavailable values and non-link rows without inferring routing", () => {
    const missingModel = {
      ...fixture,
      other_attempts: 2,
      mappings: [{ ...fixture.mappings[0], model: "", provider: "" }],
    } as UsageModelMappingDistribution

    renderMappings({ data: missingModel })

    expect(screen.getByText("Actual model unavailable")).toBeInTheDocument()
    expect(screen.getByText("Provider unavailable")).toBeInTheDocument()
    expect(screen.getByText("Other observed attempts · 2")).toBeInTheDocument()
    expect(screen.queryByRole("link")).not.toBeInTheDocument()
  })

  it("does not present a zero cost when alias coverage is absent", () => {
    renderMappings({ data: { ...fixture, observed_alias_attempts: 0, missing_alias_attempts: 6, alias_coverage: 0, observed_total_cost: 0, mappings: [] } as UsageModelMappingDistribution })
    expect(screen.getByText("Cost · No aliases")).toBeInTheDocument()
  })

  it("keeps a refresh failure visible when stale mapping data is available", () => {
    const onRetry = vi.fn()
    renderMappings({ summaryError: new Error("refresh failed"), onRetrySummary: onRetry })

    expect(screen.getByRole("alert")).toHaveTextContent("Refresh failed. Showing last result.")
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it("keeps retry available when stale data contains no attempts", () => {
    const onRetry = vi.fn()
    const emptySummary = { ...summaryFixture, total_attempts: 0, observed_alias_attempts: 0, missing_alias_attempts: 0, alias_coverage: 0, displayed_mappings: 0 }
    renderMappings({ summary: emptySummary, data: undefined, summaryError: new Error("refresh failed"), onRetrySummary: onRetry })

    expect(screen.getByText("No attempts in 24h")).toBeInTheDocument()
    expect(screen.getByRole("alert")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it("keeps mapping rows unloaded while collapsed and shows a loading state after expansion", async () => {
    const onExpandedChange = vi.fn()
    renderMappings({ data: undefined, onExpandedChange, isDetailsLoading: true })

    expect(screen.queryByText("Share of attempts with an alias")).not.toBeInTheDocument()
    fireEvent.click(screen.getByText("Model mappings").closest("summary")!)

    await waitFor(() => expect(onExpandedChange).toHaveBeenCalledWith(true))
    expect(screen.getByLabelText("Loading model mapping details")).toBeVisible()
  })
})
