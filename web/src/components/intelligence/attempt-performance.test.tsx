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
  const counts = Array<number>(24).fill(0)
  counts[0] = Math.floor(samples / 2)
  counts[23] = samples - counts[0]
  return { population_count: population, sample_count: samples, coverage: population === 0 ? null : samples / population, p50, p95, histogram: samples > 0 ? { upper_bound: p95 ?? 0, counts } : null }
}

function summary(): UsageAttemptPerformanceSummary {
  return {
    successful_attempts: 18,
    failed_attempts: 2,
    successful_execution: { generating_streaming: 10, non_generating: 2, non_streaming: 1, unknown: 5 },
    latency_ms: { successful: metric(18, 18, 500, 9_000), failed: metric(2, 1, 12_000, 12_000) },
    ttft_ms: { generating_streaming: metric(10, 8, 120, 1_500), unknown_execution: metric(5, 0, null, null) },
    output_tps: { generating_streaming: metric(10, 7, 42, 88) },
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
  it("colors equal sample shares equally across different request volumes and plots the full observed tail", () => {
    const data = performance()
    const dense = Array<number>(24).fill(0)
    dense[4] = 500
    dense[23] = 500
    const sparse = Array<number>(24).fill(0)
    sparse[4] = 5
    sparse[23] = 5
    data.models.items = [
      { ...data.models.items[0], value: "busy", label: "Busy model", attempt_count: 1000, latency_ms: { ...data.latency_ms, successful: { ...metric(1000, 1000, 500, 1000), histogram: { upper_bound: 2000, counts: dense } } } },
      { ...data.models.items[0], value: "partial", label: "Partial model", attempt_count: 1000, latency_ms: { ...data.latency_ms, successful: { ...metric(1000, 10, 500, 1000), histogram: { upper_bound: 2000, counts: sparse } } } },
    ]
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude"]} onSelectProvider={vi.fn()} provider="claude" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)
    const models = screen.getByRole("region", { name: "Models" })
    const busy = within(models).getByRole("img", { name: /Busy model/ })
    const partial = within(models).getByRole("img", { name: /Partial model/ })
    expect(busy.querySelectorAll("[data-heatmap-bin]")).toHaveLength(24)
    expect(busy.querySelector('[data-heatmap-bin="4"]')).toHaveAttribute("data-share", "0.5")
    expect(partial.querySelector('[data-heatmap-bin="4"]')).toHaveAttribute("data-share", "0.5")
    expect(busy.querySelector<HTMLElement>('[data-heatmap-bin="4"]')?.style.opacity).toBe(partial.querySelector<HTMLElement>('[data-heatmap-bin="4"]')?.style.opacity)
    expect(partial.querySelector('[data-heatmap-bin="5"]')).toHaveStyle({ opacity: "0" })
    expect(within(models).getByLabelText("Shared linear axis from zero to 2s")).toBeVisible()
    expect(busy.querySelector('[data-percentile="p95"]')).toHaveStyle({ left: "50%" })
  })

  it("opens interval counts and shares for keyboard and touch inspection, qualifying a small sample", () => {
    const data = performance()
    const counts = Array<number>(24).fill(0)
    counts[2] = 2
    counts[23] = 1
    data.models.items[0].latency_ms.successful = { ...metric(5, 3, 25, 240), histogram: { upper_bound: 240, counts } }
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude"]} onSelectProvider={vi.fn()} provider="claude" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)
    const models = screen.getByRole("region", { name: "Models" })
    fireEvent.click(within(models).getByRole("button", { name: "View sample distribution for sonnet" }))
    const details = screen.getByRole("dialog", { name: "sonnet sample distribution" })
    expect(within(details).getByText("3 valid samples · 24 equal intervals")).toBeVisible()
    expect(within(details).getByText(/Few samples/)).toBeVisible()
    const bin = within(details).getByText("20–30 ms").closest("tr")!
    expect(within(bin).getByText("2", { exact: true })).toBeVisible()
    expect(within(bin).getByText("66.7%", { exact: true })).toBeVisible()
    fireEvent.keyDown(details, { key: "Escape" })
    expect(screen.queryByRole("dialog", { name: "sonnet sample distribution" })).not.toBeInTheDocument()
  })

  it("keeps the provider selector accessible while loading or showing an error", () => {
    const onSelect = vi.fn()
    const { rerender } = render(<AttemptPerformance onRetryProviders={vi.fn()} provider="claude" providers={["claude", "openai"]} onSelectProvider={onSelect} data={undefined} isLoading error={null} onRetry={vi.fn()} />)
    fireEvent.keyDown(screen.getByRole("combobox", { name: "Performance provider" }), { key: "ArrowDown" })
    fireEvent.click(screen.getByRole("option", { name: "openai" }))
    expect(onSelect).toHaveBeenCalledWith("openai")
    rerender(<AttemptPerformance onRetryProviders={vi.fn()} provider="openai" providers={["claude", "openai"]} onSelectProvider={onSelect} data={undefined} isLoading={false} error={new Error("failed")} onRetry={vi.fn()} />)
    expect(screen.getByRole("combobox", { name: "Performance provider" })).toHaveTextContent("openai")
    expect(screen.getByText("Couldn't load attempt performance")).toBeVisible()
  })

  it("shows an actual-model percentile chart by default and preserves the exact slow-evidence window", () => {
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="claude" data={performance()} isLoading={false} error={null} onRetry={vi.fn()} />)

    expect(screen.getByRole("heading", { name: "Attempt performance" })).toBeInTheDocument()
    expect(screen.getByText("Last 24h")).toBeInTheDocument()
    expect(within(screen.getByLabelText("Performance metric")).getByRole("button", { name: "Latency" })).toHaveAttribute("aria-pressed", "true")
    expect(within(screen.getByLabelText("Compare by")).getByRole("button", { name: "Models" })).toHaveAttribute("aria-pressed", "true")

    const models = screen.getByRole("region", { name: "Models" })
    expect(within(models).getByLabelText("Shared linear axis from zero to 9s")).toBeInTheDocument()
    expect(within(models).getByRole("img", { name: "sonnet: p50 0.5s, p95 9s" })).toBeInTheDocument()
    const sampleDetails = within(models).getByLabelText("Sample details for sonnet")
    expect(sampleDetails.closest("details")).not.toHaveAttribute("open")
    expect(within(models).getByText("18 of 18")).not.toBeVisible()
    fireEvent.click(sampleDetails)
    expect(within(models).getByText("18 of 18")).toBeVisible()
    expect(within(models).getByText("Other or unavailable").parentElement).toHaveTextContent("3 attempts")

    const slowLink = screen.getByRole("link", { name: "Inspect successful attempts at or above p95 latency" })
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"provider":"claude"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"minLatencyMS":"9000"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"windowEnd":"2026-09-07T12:00:00.123456789Z"'))
    expect(slowLink).toHaveAttribute("data-search", expect.stringContaining('"result":"success"'))
  })

  it("switches the visible comparison between metrics and dimensions", () => {
    const data = performance()
    data.models.items[0] = { ...data.models.items[0], ttft_ms: { ...data.models.items[0].ttft_ms, generating_streaming: metric(10, 0, null, null) } }
    data.accounts.items[0] = { ...data.accounts.items[0], output_tps: { generating_streaming: metric(10, 7, 42, 42) } }
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="claude" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)

    const metrics = screen.getByLabelText("Performance metric")
    const dimensions = screen.getByLabelText("Compare by")
    fireEvent.click(within(metrics).getByRole("button", { name: "TTFT" }))

    const models = screen.getByRole("region", { name: "Models" })
    expect(within(models).getByLabelText("Shared linear axis from zero to 0s")).toBeInTheDocument()
    expect(within(models).getByText("No valid samples")).toBeInTheDocument()
    expect(screen.queryByRole("dialog", { name: "Unknown execution TTFT" })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "About TTFT execution unknown" }))
    const unknown = screen.getByRole("dialog", { name: "Unknown execution TTFT" })
    expect(within(unknown).getByText("0 of 5")).toHaveAttribute("title", "0 of 5 attempts sampled")
    fireEvent.keyDown(unknown, { key: "Escape" })
    expect(screen.queryByRole("dialog", { name: "Unknown execution TTFT" })).not.toBeInTheDocument()

    fireEvent.click(within(dimensions).getByRole("button", { name: "Providers" }))
    const providers = screen.getByRole("region", { name: "Providers" })
    expect(within(providers).getByLabelText("Shared linear axis from zero to 1.5s")).toBeInTheDocument()

    fireEvent.click(within(metrics).getByRole("button", { name: "Output TPS" }))
    fireEvent.click(within(dimensions).getByRole("button", { name: "Accounts" }))
    const accounts = screen.getByRole("region", { name: "Accounts" })
    expect(within(accounts).getByLabelText("Shared linear axis from zero to 42.0 tok/s")).toBeInTheDocument()
    const equalPercentiles = within(accounts).getByRole("img", { name: "Claude Primary: p50 42.0 tok/s, p95 42.0 tok/s" })
    expect(equalPercentiles.querySelector('[data-percentile="p50"]')).toHaveStyle({ left: "100%" })
    expect(equalPercentiles.querySelector('[data-percentile="p95"]')).toHaveStyle({ left: "100%" })
  })

  it("keeps partial and missing coverage visible while complete coverage stays in sample details", () => {
    const data = performance()
    data.models.items = [
      { ...data.models.items[0], value: "partial", label: "Partial model", latency_ms: { ...data.latency_ms, successful: metric(1000, 999, 500, 9000) } },
      { ...data.models.items[0], value: "missing", label: "Missing model", latency_ms: { ...data.latency_ms, successful: metric(0, 0, null, null) } },
    ]
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="claude" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)
    const models = screen.getByRole("region", { name: "Models" })
    expect(within(models).getByText("<100% coverage", { exact: true })).toBeVisible()
    expect(within(models).getByText("Coverage unavailable", { exact: true })).toBeVisible()
  })

  it("keeps failed latency separate with aggregate and dimension drill-down links", () => {
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="claude" data={performance()} isLoading={false} error={null} onRetry={vi.fn()} />)

    const failedSummary = screen.getByText(/Failed latency/)
    fireEvent.click(failedSummary)
    const failedOverall = screen.getByRole("link", { name: "Inspect failed attempts at or above p95 latency" })
    expect(failedOverall).toHaveAttribute("data-search", expect.stringContaining('"result":"failed"'))
    expect(failedOverall).toHaveAttribute("data-search", expect.stringContaining('"minLatencyMS":"12000"'))
    const failedModel = screen.getByRole("link", { name: "Inspect sonnet failed attempts at or above p95 latency" })
    expect(failedModel).toHaveAttribute("data-search", expect.stringContaining('"model":"sonnet"'))
  })

  it("keeps unscoped provider throughput numeric while requiring a provider for model and account axes", () => {
    const data = performance()
    data.providers.items[0].output_tps.generating_streaming = metric(10, 5, 42, 42)
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)

    fireEvent.click(within(screen.getByLabelText("Performance metric")).getByRole("button", { name: "Output TPS" }))
    expect(screen.getByLabelText("Overall selected metric")).not.toBeVisible()
    expect(within(screen.getByRole("region", { name: "Models" })).getByText("Select a provider.")).toBeInTheDocument()

    fireEvent.click(within(screen.getByLabelText("Compare by")).getByRole("button", { name: "Providers" }))
    const providers = screen.getByRole("region", { name: "Providers" })
    expect(within(providers).queryByRole("img")).not.toBeInTheDocument()
    expect(within(providers).getAllByText("42.0 tok/s")).toHaveLength(2)
    expect(within(providers).getByText("Select a provider.")).toBeInTheDocument()

    fireEvent.click(within(screen.getByLabelText("Compare by")).getByRole("button", { name: "Accounts" }))
    expect(within(screen.getByRole("region", { name: "Accounts" })).getByText("Select a provider.")).toBeInTheDocument()
  })

  it("plots observed zero percentiles instead of treating them as missing", () => {
    const data = performance()
    data.models.items[0] = { ...data.models.items[0], latency_ms: { ...data.models.items[0].latency_ms, successful: metric(18, 18, 0, 0) } }
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="claude" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)

    const chart = within(screen.getByRole("region", { name: "Models" })).getByRole("img", { name: "sonnet: p50 0s, p95 0s" })
    expect(chart.querySelector('[data-percentile="p50"]')).toHaveStyle({ left: "0%" })
    expect(chart.querySelector('[data-percentile="p95"]')).toHaveStyle({ left: "0%" })
  })

  it("keeps unavailable, empty, initial error, and stale retry states explicit", () => {
    const retry = vi.fn()
    const { rerender } = render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="" data={undefined} isLoading={false} error={new Error("offline")} onRetry={retry} />)
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(retry).toHaveBeenCalledTimes(1)

    const empty = performance()
    empty.total_attempts = 0
    rerender(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="" data={empty} isLoading={false} error={null} onRetry={retry} />)
    expect(screen.getByText("No providers in 24h")).toBeInTheDocument()

    rerender(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude", "openai"]} onSelectProvider={vi.fn()} provider="" data={performance()} isLoading={false} error={new Error("refresh")} onRetry={retry} />)
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument()
  })
})
