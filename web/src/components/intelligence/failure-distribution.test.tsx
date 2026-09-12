import { cleanup, render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageFailureDistribution } from "@/types/api"
import { FailureDistribution } from "./failure-distribution"

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, search, ...props }: { children: React.ReactNode; search: Record<string, string> }) => (
    <a href={`?${new URLSearchParams(search).toString()}`} {...props}>{children}</a>
  ),
}))

afterEach(cleanup)

const emptyBreakdown = { items: [], other_count: 0 }
const distribution: UsageFailureDistribution = {
  window_start: "2026-09-06T12:00:00.123456789Z",
  window_end: "2026-09-07T12:00:00.123456789Z",
  total_failures: 3,
  categories: { items: [{ value: "4xx", label: "4XX", category: "4xx", count: 2 }], other_count: 1 },
  statuses: { items: [{ value: "429", label: "HTTP 429", category: "4xx", count: 2 }], other_count: 1 },
  providers: { items: [{ value: "claude", label: "claude", count: 3 }], other_count: 0 },
  accounts: { items: [{ value: "auth-1", label: "Claude Primary", count: 2 }], other_count: 1 },
  models: { items: [{ value: "sonnet", label: "sonnet", count: 3 }], other_count: 0 },
  endpoints: { items: [{ value: "/v1/messages", label: "/v1/messages", count: 3 }], other_count: 0 },
}

describe("FailureDistribution", () => {
  it("opens first-page Request Evidence with the selected breakdown and frozen window", async () => {
    render(<FailureDistribution provider="claude" data={distribution} isLoading={false} error={null} onRetry={vi.fn()} />)

    expect(screen.getByText("Failure breakdown").closest("details")).not.toHaveAttribute("open")
    const allFailures = screen.getByRole("link", { name: "View failures" })
    expect(allFailures).toHaveAttribute("href", expect.stringContaining("result=failed"))
    expect(allFailures).toHaveAttribute("href", expect.stringContaining("windowEnd=2026-09-07T12%3A00%3A00.123456789Z"))
    await userEvent.click(screen.getByText("Failure breakdown"))
    const statusLink = screen.getByRole("link", { name: "Inspect HTTP 429 failures" })
    expect(statusLink).toHaveAttribute("href", expect.stringContaining("provider=claude"))
    expect(statusLink).toHaveAttribute("href", expect.stringContaining("status=429"))
    expect(statusLink).toHaveAttribute("href", expect.stringContaining("result=failed"))
    expect(statusLink).toHaveAttribute("href", expect.stringContaining("windowEnd=2026-09-07T12%3A00%3A00.123456789Z"))
    expect(screen.getByRole("region", { name: "Exact statuses" })).toBeInTheDocument()
    expect(statusLink).toHaveTextContent("66.7%")
    expect(statusLink).toHaveAttribute("title", "HTTP 429: 2 of 3 failed attempts")
    expect(within(screen.getByRole("region", { name: "Exact statuses" })).getByText("Other or unavailable")).toBeInTheDocument()
    expect(within(screen.getByRole("region", { name: "Exact statuses" })).queryByRole("button")).not.toBeInTheDocument()
  })

  it("keeps initial API error and successful empty state distinct", async () => {
    const retry = vi.fn()
    const { rerender } = render(<FailureDistribution provider="" data={undefined} isLoading={false} error={new Error("offline")} onRetry={retry} />)
    expect(screen.getByText("Couldn't load failure distribution")).toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(retry).toHaveBeenCalledTimes(1)

    rerender(<FailureDistribution provider="" data={{ ...distribution, total_failures: 0, categories: emptyBreakdown }} isLoading={false} error={null} onRetry={retry} />)
    expect(screen.getByText("No failures in 24h")).toBeInTheDocument()
    expect(screen.queryByText("Couldn't load failure distribution")).not.toBeInTheDocument()
    expect(screen.queryByText("Failure breakdown")).not.toBeInTheDocument()
  })

  it("accounts for lower-ranked rows before expanding all returned breakdown items", async () => {
    const categories = {
      items: [
        { value: "1xx", label: "1XX", count: 8 },
        { value: "2xx", label: "2XX", count: 7 },
        { value: "3xx", label: "3XX", count: 6 },
        { value: "4xx", label: "4XX", count: 5 },
        { value: "5xx", label: "5XX", count: 4 },
        { value: "unknown", label: "Unknown status", count: 3 },
      ],
      other_count: 2,
    }
    render(<FailureDistribution provider="" data={{ ...distribution, total_failures: 35, categories }} isLoading={false} error={null} onRetry={vi.fn()} />)

    await userEvent.click(screen.getByText("Failure breakdown"))
    const section = screen.getByRole("region", { name: "Status families" })
    expect(section).not.toHaveTextContent("5XX")
    expect(section).toHaveTextContent("7 attempts")
    expect(section).toHaveTextContent("Other or unavailable2")
    await userEvent.click(within(section).getByRole("button", { name: /Show 2 more/ }))
    expect(section).toHaveTextContent("5XX")
    expect(section).toHaveTextContent("Unknown status")
    expect(section).toHaveTextContent("Other or unavailable2")
    expect(within(section).getAllByRole("link")).toHaveLength(6)
    expect(within(section).queryByRole("button")).not.toBeInTheDocument()
  })
})
