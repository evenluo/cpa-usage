import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageEventsPage } from "@/types/api"
import { RequestEvidence } from "./request-evidence"

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, search }: { children: React.ReactNode; search?: { provider?: string } }) => (
    <a href={search?.provider ? `/requests?provider=${encodeURIComponent(search.provider)}` : "/requests"}>{children}</a>
  ),
}))

afterEach(cleanup)

const populatedPage: UsageEventsPage = {
  events: [{
    id: 1,
    timestamp: "2026-08-27T00:00:00Z",
    model: "gpt-5",
    source: "account",
    failed: false,
    latency_ms: 10,
    ttft_ms: 2,
    output_tps: 100,
    tokens: { output_tokens: 1, total_tokens: 2 },
  }],
  total_count: 1,
  page: 1,
  page_size: 1,
  total_pages: 1,
}

describe("RequestEvidence", () => {
  it("shows a scoped retry for an initial failure", async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()
    render(<RequestEvidence provider="" data={undefined} isLoading={false} isRefreshing={false} error={new Error("offline")} onRetry={onRetry} />)

    expect(screen.getByText("Failed to load request evidence")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Retry request evidence" }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it("keeps complete stale data visible when refresh fails", async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()
    render(<RequestEvidence provider="claude" data={populatedPage} isLoading={false} isRefreshing={false} error={new Error("refresh failed")} onRetry={onRetry} />)

    expect(screen.getByRole("region", { name: "Latest upstream attempt" })).toBeInTheDocument()
    expect(screen.queryByText("Failed to load request evidence")).not.toBeInTheDocument()
    expect(screen.getByRole("link", { name: "View all attempts" })).toHaveAttribute("href", "/requests?provider=claude")
    await user.click(screen.getByRole("button", { name: "Retry refresh" }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it("distinguishes a successful empty response from loading and failure", () => {
    render(<RequestEvidence provider="" data={{ ...populatedPage, events: [], total_count: 0 }} isLoading={false} isRefreshing={false} error={null} onRetry={vi.fn()} />)
    expect(screen.getByText("No recent request evidence")).toBeInTheDocument()
  })
})
