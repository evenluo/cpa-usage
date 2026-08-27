import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageEventsPage } from "@/types/api"

vi.mock("@tanstack/react-router", () => ({
  createLazyFileRoute: () => (options: object) => ({ ...options, useSearch: () => ({ provider: "" }) }),
  Link: ({ children }: { children: React.ReactNode }) => <a href="/">{children}</a>,
}))

vi.mock("@/hooks/useEvents", () => ({
  useEvents: vi.fn(),
}))

import { useEvents } from "@/hooks/useEvents"
import { RequestsPage } from "./requests.lazy"

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function eventsPage(page: number, provider: string): UsageEventsPage {
  return {
    events: Array.from({ length: 2 }, (_, index) => ({
      id: page * 10 + index,
      timestamp: "2026-08-27T00:00:00Z",
      model: `${provider || "all"}-model-${index + 1}`,
      source: provider || "all",
      failed: false,
      latency_ms: 10,
      ttft_ms: 2,
      output_tps: 100,
      tokens: { output_tokens: 1, total_tokens: 2 },
    })),
    total_count: 20,
    page,
    page_size: 10,
    total_pages: 2,
  }
}

describe("RequestsPage provider scope", () => {
  it("queries the new provider at page one and clears page-local selection on the first scoped render", async () => {
    vi.mocked(useEvents).mockImplementation((_range, _pageSize, provider = "", page = 1) => ({
      data: eventsPage(page, provider),
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }) as never)
    const user = userEvent.setup()
    const { rerender } = render(<RequestsPage provider="claude" />)

    await user.click(screen.getByRole("button", { name: "Next page" }))
    await user.click(screen.getByRole("button", { name: "Select request 21" }))
    expect(screen.getByText("Page 2 of 2")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Select request 21" })).toHaveAttribute("aria-pressed", "true")

    rerender(<RequestsPage provider="codex" />)

    const calls = vi.mocked(useEvents).mock.calls
    expect(calls[calls.length - 1]?.slice(0, 4)).toEqual(["24h", 10, "codex", 1])
    expect(screen.getByText("Page 1 of 2")).toBeInTheDocument()
    expect(screen.getByTestId("request-provider-scope")).toHaveTextContent("Provider: codex")
    expect(screen.getByRole("button", { name: "Select request 10" })).toHaveAttribute("aria-pressed", "true")
  })
})
