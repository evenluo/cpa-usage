import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageEventsPage } from "@/types/api"
import { ApiError } from "@/lib/api"

vi.mock("@tanstack/react-router", () => ({
  createLazyFileRoute: () => (options: object) => ({
    ...options,
    useSearch: () => ({ provider: "", model: "", result: "" }),
    useNavigate: () => vi.fn(),
  }),
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
      request_id: `request-${index + 1}`,
      source: provider || "all",
      failed: false,
      latency_ms: 10,
      ttft_ms: 2,
      output_tps: 100,
      tokens: { output_tokens: 1, total_tokens: 2 },
    })),
    window_end: "2026-09-07T12:00:00.123456789Z",
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
    await user.click(screen.getByRole("button", { name: "Select attempt 21" }))
    expect(screen.getByText("Page 2 of 2")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Select attempt 21" })).toHaveAttribute("aria-pressed", "true")

    rerender(<RequestsPage provider="codex" />)

    const calls = vi.mocked(useEvents).mock.calls
    expect(calls[calls.length - 1]?.slice(0, 4)).toEqual(["24h", 10, "codex", 1])
    expect(screen.getByText("Page 1 of 2")).toBeInTheDocument()
    expect(screen.getByTestId("request-provider-scope")).toHaveTextContent("Provider: codex")
    expect(screen.getByRole("button", { name: "Select attempt 10" })).toHaveAttribute("aria-pressed", "true")
  })

  it("passes model and result scope to the existing evidence query", async () => {
    vi.mocked(useEvents).mockReturnValue({
      data: eventsPage(1, "claude"),
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)

    const onFiltersChange = vi.fn()
    const user = userEvent.setup()
    render(<RequestsPage provider="claude" model="gpt-5" result="failed" onFiltersChange={onFiltersChange} />)

    const calls = vi.mocked(useEvents).mock.calls
    const call = calls[calls.length - 1]
    expect(call?.[5]).toEqual({ model: "gpt-5", modelAlias: "", account: "", endpoint: "", status: "", requestId: "", windowEnd: "", result: "failed" })
    expect(screen.getByDisplayValue("gpt-5")).toBeInTheDocument()
    expect(screen.getByDisplayValue("Failed attempts")).toBeInTheDocument()

    await user.clear(screen.getByLabelText("Actual model"))
    await user.type(screen.getByLabelText("Actual model"), "claude-sonnet")
    await user.click(screen.getByRole("button", { name: "Apply model" }))
    expect(onFiltersChange).toHaveBeenCalledWith({ model: "claude-sonnet", result: "failed" })
  })

  it("passes the frozen diagnostic selection and clears page-local state when scope changes", async () => {
    vi.mocked(useEvents).mockImplementation((_range, _pageSize, provider = "", page = 1) => ({
      data: eventsPage(page, provider), isLoading: false, error: null, refetch: vi.fn(),
    }) as never)
    const onFiltersChange = vi.fn()
    const { rerender } = render(
      <RequestsPage
        provider="claude"
        modelAlias="sonnet-route"
        account="auth-1"
        endpoint="/v1/messages"
        status="4xx"
        windowEnd="2026-09-07T12:00:00.123456789Z"
        result="failed"
        onFiltersChange={onFiltersChange}
      />,
    )

    const diagnosticCalls = vi.mocked(useEvents).mock.calls
    expect(diagnosticCalls[diagnosticCalls.length - 1]?.[5]).toEqual({
      model: "", modelAlias: "sonnet-route", account: "auth-1", endpoint: "/v1/messages", status: "4xx", requestId: "",
      windowEnd: "2026-09-07T12:00:00.123456789Z", result: "failed",
    })
    expect(screen.getByLabelText("Diagnostic filters")).toHaveTextContent("Account: auth-1")
    expect(screen.getByLabelText("Diagnostic filters")).toHaveTextContent("Observed alias: sonnet-route")

    rerender(<RequestsPage provider="openai" status="500" result="failed" onFiltersChange={onFiltersChange} />)
    const resetCalls = vi.mocked(useEvents).mock.calls
    expect(resetCalls[resetCalls.length - 1]?.slice(0, 4)).toEqual(["24h", 10, "openai", 1])
    expect(screen.getByRole("button", { name: "Select attempt 10" })).toHaveAttribute("aria-pressed", "true")

    await userEvent.click(screen.getByRole("button", { name: "Clear diagnostic filters" }))
    expect(onFiltersChange).toHaveBeenCalledWith({ model: "", modelAlias: "", result: "failed", account: "", endpoint: "", status: "", windowEnd: "" })
  })

  it("replaces the current filters with a fixed correlated-attempt selection", async () => {
    vi.mocked(useEvents).mockReturnValue({ data: eventsPage(1, "claude"), isLoading: false, error: null, refetch: vi.fn() } as never)
    const onCorrelatedAttempts = vi.fn()
    const user = userEvent.setup()
    render(
      <RequestsPage
        provider="claude"
        model="sonnet"
        modelAlias="sonnet-route"
        account="auth-1"
        endpoint="/v1/messages"
        status="429"
        result="failed"
        onCorrelatedAttempts={onCorrelatedAttempts}
      />,
    )

    await user.click(screen.getByRole("button", { name: "View correlated attempts" }))
    expect(onCorrelatedAttempts).toHaveBeenCalledWith({
      provider: "claude", model: "", modelAlias: "", account: "", endpoint: "", status: "", requestId: "request-1",
      windowEnd: "2026-09-07T12:00:00.123456789Z", result: "",
    })
  })

  it("shows correlation limits and omits the action when request ID is missing", () => {
    vi.mocked(useEvents).mockReturnValue({ data: eventsPage(1, "claude"), isLoading: false, error: null, refetch: vi.fn() } as never)
    const { rerender } = render(<RequestsPage provider="claude" requestId="request-42" windowEnd="2026-09-07T12:00:00.123456789Z" />)
    expect(screen.getByLabelText("Correlation scope")).toHaveTextContent("other providers are not included")
    expect(screen.getByLabelText("Correlation scope")).toHaveTextContent("Historical data may omit attempts")

    const missing = eventsPage(1, "claude")
    missing.events = [{ ...missing.events[0], request_id: undefined }]
    vi.mocked(useEvents).mockReturnValue({ data: missing, isLoading: false, error: null, refetch: vi.fn() } as never)
    rerender(<RequestsPage provider="claude" />)
    expect(screen.queryByRole("button", { name: "View correlated attempts" })).not.toBeInTheDocument()
  })

  it("distinguishes invalid filters, API failure, and an empty successful page", () => {
    vi.mocked(useEvents).mockReturnValue({ data: undefined, isLoading: false, error: new ApiError(400, "invalid status"), refetch: vi.fn() } as never)
    const { rerender } = render(<RequestsPage provider="" status="broken" />)
    expect(screen.getByText("Invalid request evidence filters")).toBeInTheDocument()

    vi.mocked(useEvents).mockReturnValue({ data: undefined, isLoading: false, error: new ApiError(500, "unavailable"), refetch: vi.fn() } as never)
    rerender(<RequestsPage provider="" status="500" />)
    expect(screen.getByText("Failed to load request evidence")).toBeInTheDocument()

    vi.mocked(useEvents).mockReturnValue({ data: { ...eventsPage(1, ""), events: [], total_count: 0, total_pages: 1 }, isLoading: false, error: null, refetch: vi.fn() } as never)
    rerender(<RequestsPage provider="" />)
    expect(screen.getByText("No recent request evidence")).toBeInTheDocument()
  })
})
