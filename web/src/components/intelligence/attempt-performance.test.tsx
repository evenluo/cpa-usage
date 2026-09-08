import { cleanup, fireEvent, render, screen, within } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageAttemptPerformance, UsageAttemptPerformanceSummary, UsagePercentileDistribution } from "@/types/api"

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, search, ...props }: { children: React.ReactNode; search: object; [key: string]: unknown }) => (
    <a href="/requests" data-search={JSON.stringify(search)} {...props}>{children}</a>
  ),
}))

import { AttemptPerformance } from "./attempt-performance"

afterEach(cleanup)

function metric(population: number, samples: number, p50: number | null, p95: number | null): UsagePercentileDistribution {
  return {
    population_count: population,
    sample_count: samples,
    coverage: population === 0 ? null : samples / population,
    p50,
    p95,
  }
}

function summary(): UsageAttemptPerformanceSummary {
  return {
    successful_attempts: 18,
    failed_attempts: 2,
    successful_execution: { generating_streaming: 10, non_generating: 2, non_streaming: 1, unknown: 5 },
    latency_ms: {
      successful: metric(18, 18, 500, 9_000),
      failed: metric(2, 1, 12_000, 12_000),
    },
    ttft_ms: {
      generating_streaming: metric(10, 8, 120, 1_500),
      unknown_execution: metric(5, 0, null, null),
    },
    output_tps: {
      generating_streaming: metric(10, 7, 42, 88),
    },
  }
}

function performance(): UsageAttemptPerformance {
  const base = summary()
  const item = { ...base, value: "sonnet", label: "sonnet", attempt_count: 20 }
  return {
    ...base,
    window_start: "2026-09-06T12:00:00Z",
    window_end: "2026-09-07T12:00:00.123456789Z",
    total_attempts: 20,
    providers: { items: [{ ...item, value: "claude", label: "claude" }], other_count: 0 },
    models: { items: [item], other_count: 3 },
    accounts: { items: [{ ...item, value: "auth-1", label: "Claude Primary" }], other_count: 0 },
  }
}

describe("AttemptPerformance", () => {
  it("shows qualified long-tail distributions and exact slow evidence selection", () => {
    render(<AttemptPerformance provider="claude" data={performance()} isLoading={false} error={null} onRetry={vi.fn()} />)

    expect(screen.getAllByText("20 attempts", { exact: false }).length).toBeGreaterThan(0)
    expect(screen.getByText("100%")).toHaveAttribute("title", "18 of 18 attempts sampled")
    expect(screen.getByText("80%")).toHaveAttribute("title", "8 of 10 attempts sampled")
    expect(screen.getByText(/2 non-generating and 1 non-streaming/)).toBeInTheDocument()
    expect(screen.getByText("Other or unavailable")).toHaveTextContent("Other or unavailable")

    const slowLink = screen.getByRole("link", { name: "Inspect successful attempts at or above p95 latency" })
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"provider":"claude"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"minLatencyMS":"9000"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"windowEnd":"2026-09-07T12:00:00.123456789Z"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"result":"success"'))
  })

  it("switches the three ordered breakdowns between qualified percentile views", () => {
    const data = performance()
    data.models.items[0] = {
      ...data.models.items[0],
      latency_ms: { ...data.models.items[0].latency_ms, successful: metric(18, 18, 400, 6_000) },
      ttft_ms: { ...data.models.items[0].ttft_ms, generating_streaming: metric(10, 0, null, null) },
    }
    data.accounts.items[0] = {
      ...data.accounts.items[0],
      latency_ms: { ...data.accounts.items[0].latency_ms, successful: metric(18, 18, 300, 3_000) },
      output_tps: { generating_streaming: metric(10, 7, 42, 42) },
    }
    render(<AttemptPerformance provider="claude" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)

    const controls = screen.getByLabelText("Performance breakdown metric")
    expect(within(controls).getByRole("button", { name: "Successful latency" })).toHaveAttribute("aria-pressed", "true")
    const providers = screen.getByRole("region", { name: "Providers" })
    const models = screen.getByRole("region", { name: "Actual models" })
    const accounts = screen.getByRole("region", { name: "Accounts" })
    expect(within(providers).getByLabelText("Shared linear axis from zero to 9s")).toBeInTheDocument()
    expect(within(models).getByLabelText("Shared linear axis from zero to 6s")).toBeInTheDocument()
    expect(within(accounts).getByLabelText("Shared linear axis from zero to 3s")).toBeInTheDocument()
    expect(within(providers).getByRole("img", { name: "claude: p50 0.5s, p95 9s" })).toBeInTheDocument()

    fireEvent.click(within(controls).getByRole("button", { name: "Failed latency" }))
    expect(within(controls).getByRole("button", { name: "Failed latency" })).toHaveAttribute("aria-pressed", "true")
    expect(screen.getAllByLabelText("Shared linear axis from zero to 12s")).toHaveLength(3)
    const failedLink = screen.getByRole("link", { name: "Inspect sonnet failed attempts at or above p95 latency" })
    expect(failedLink).toHaveAttribute("data-search", expect.stringContaining('"result":"failed"'))
    expect(failedLink).toHaveAttribute("data-search", expect.stringContaining('"minLatencyMS":"12000"'))
    expect(failedLink).toHaveAttribute("data-search", expect.stringContaining('"model":"sonnet"'))

    fireEvent.click(within(controls).getByRole("button", { name: "TTFT" }))
    expect(within(providers).getByLabelText("Shared linear axis from zero to 1.5s")).toBeInTheDocument()
    expect(within(models).getByLabelText("Shared linear axis from zero to 0s")).toBeInTheDocument()
    expect(within(models).getByText("No valid samples")).toBeInTheDocument()
    expect(within(accounts).getByText("8 / 10 samples")).toBeInTheDocument()
    expect(within(accounts).getByText("80% coverage")).toHaveAttribute("title", "8 of 10 attempts sampled")

    fireEvent.click(within(controls).getByRole("button", { name: "Output TPS" }))
    expect(within(providers).getByLabelText("Shared linear axis from zero to 88.0 tok/s")).toBeInTheDocument()
    expect(within(models).getByLabelText("Shared linear axis from zero to 88.0 tok/s")).toBeInTheDocument()
    expect(within(accounts).getByLabelText("Shared linear axis from zero to 42.0 tok/s")).toBeInTheDocument()
    expect(within(providers).getByRole("img", { name: "claude: p50 42.0 tok/s, p95 88.0 tok/s" })).toBeInTheDocument()
    const equalPercentiles = within(accounts).getByRole("img", { name: "Claude Primary: p50 42.0 tok/s, p95 42.0 tok/s" })
    expect(equalPercentiles.querySelector('[data-percentile="p50"]')).toHaveStyle({ left: "100%" })
    expect(equalPercentiles.querySelector('[data-percentile="p95"]')).toHaveStyle({ left: "100%" })
  })

  it("keeps cross-provider throughput numeric and requires a provider for model and account comparisons", () => {
    const data = performance()
    data.providers.items[0].output_tps.generating_streaming = metric(10, 5, 42, 42)
    render(<AttemptPerformance provider="" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)

    fireEvent.click(screen.getByRole("button", { name: "Output TPS" }))
    const providers = screen.getByRole("region", { name: "Providers" })
    expect(within(providers).queryByRole("img")).not.toBeInTheDocument()
    expect(within(providers).getAllByText("42.0 tok/s")).toHaveLength(2)
    expect(within(providers).getByText(/select one provider for a shared comparison axis/i)).toBeInTheDocument()
    const models = screen.getByRole("region", { name: "Actual models" })
    expect(within(models).getByText("Select a provider to compare actual models.")).toBeInTheDocument()
    expect(within(models).getByText("Other or unavailable").parentElement).toHaveTextContent("3 attempts")
    expect(within(screen.getByRole("region", { name: "Accounts" })).getByText("Select a provider to compare accounts.")).toBeInTheDocument()
  })

  it("keeps unavailable, empty, initial error, and stale retry states explicit", () => {
    const retry = vi.fn()
    const { rerender } = render(<AttemptPerformance provider="" data={undefined} isLoading={false} error={new Error("offline")} onRetry={retry} />)
    fireEvent.click(screen.getByRole("button", { name: "Retry attempt performance" }))
    expect(retry).toHaveBeenCalledTimes(1)

    const empty = performance()
    empty.total_attempts = 0
    rerender(<AttemptPerformance provider="" data={empty} isLoading={false} error={null} onRetry={retry} />)
    expect(screen.getByText("No attempts in the last 24 hours")).toBeInTheDocument()

    rerender(<AttemptPerformance provider="" data={performance()} isLoading={false} error={new Error("refresh") } onRetry={retry} />)
    expect(screen.getByRole("button", { name: "Retry refresh" })).toBeInTheDocument()
    expect(screen.getByText("0%")).toHaveAttribute("title", "0 of 5 attempts sampled")
    expect(screen.getAllByText("Select a provider for comparable throughput.")).toHaveLength(1)
  })
})
