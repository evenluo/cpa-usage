import { cleanup, fireEvent, render, screen } from "@testing-library/react"
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
      unknown_execution: metric(5, 2, 30, 35),
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
    expect(screen.getByText("18/18 · 100%")).toBeInTheDocument()
    expect(screen.getByText("8/10 · 80%")).toBeInTheDocument()
    expect(screen.getByText(/2 non-generating and 1 non-streaming/)).toBeInTheDocument()
    expect(screen.getByText("Other or unavailable")).toHaveTextContent("Other or unavailable")

    const slowLink = screen.getByRole("link", { name: "Inspect successful attempts at or above p95 latency" })
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"provider":"claude"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"minLatencyMS":"9000"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"windowEnd":"2026-09-07T12:00:00.123456789Z"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"result":"success"'))
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
    expect(screen.getByText("0/5 · 0%")).toBeInTheDocument()
    expect(screen.getAllByText("Select a provider for comparable throughput.")).toHaveLength(1)
    expect(screen.getAllByText(/TPS Select provider/)).toHaveLength(2)
  })
})
