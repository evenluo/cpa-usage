import { cleanup, fireEvent, render, screen, within } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageAttemptPerformance, UsageAttemptPerformanceSummary, UsageOutputTPSDistribution, UsagePercentileDistribution } from "@/types/api"

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

const outputTPSBandEdges = [0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 150, 200, 300]

function outputTPS(population: number, samples: number, p50: number | null, p10: number | null, counts?: number[]): UsageOutputTPSDistribution {
  const bands = counts ?? Array<number>(outputTPSBandEdges.length).fill(0)
  const bandOf = (value: number) => outputTPSBandEdges.filter((edge) => edge <= value).length - 1
  if (counts === undefined && samples > 0 && p50 !== null && p10 !== null) {
    bands[bandOf(p10)] += Math.floor(samples / 2)
    bands[bandOf(p50)] += samples - Math.floor(samples / 2)
  }
  return { population_count: population, sample_count: samples, coverage: population === 0 ? null : samples / population, p50, p10, bands: samples > 0 ? { edges: outputTPSBandEdges, counts: bands } : null }
}

function summary(): UsageAttemptPerformanceSummary {
  return {
    successful_attempts: 18,
    failed_attempts: 2,
    successful_execution: { generating_streaming: 10, non_generating: 2, non_streaming: 1, unknown: 5 },
    latency_ms: { successful: metric(18, 18, 500, 9_000), failed: metric(2, 1, 12_000, 12_000) },
    ttft_ms: { generating_streaming: metric(10, 8, 120, 1_500), unknown_execution: metric(5, 0, null, null) },
    output_tps: { generating_streaming: outputTPS(10, 7, 42, 18) },
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
    data.accounts.items[0] = { ...data.accounts.items[0], output_tps: { generating_streaming: outputTPS(10, 7, 42, 42) } }
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
    expect(within(accounts).getByLabelText("Fixed throughput bands from zero to 300+ tok/s")).toHaveTextContent("10 tok/s bands to 100, then 150, 200, 300+")
    const equalPercentiles = within(accounts).getByRole("img", { name: "Claude Primary: p50 42.0 tok/s, p10 42.0 tok/s" })
    // 42 tok/s sits 20% into band 4 of 14: (4 + 0.2) / 14.
    expect(equalPercentiles.querySelector('[data-percentile="p50"]')).toHaveStyle({ left: "30%" })
    expect(equalPercentiles.querySelector('[data-percentile="p10"]')).toHaveStyle({ left: "30%" })
    expect(screen.getAllByLabelText("Overall selected metric")[0]).toHaveTextContent("p10 18.0 tok/s")
  })

  it("keeps slow throughput legible in fixed 10 tok/s bands beside a fast model and leaves the open band unstretched", () => {
    const data = performance()
    const slow = Array<number>(14).fill(0)
    slow[2] = 60
    slow[5] = 40
    const fast = Array<number>(14).fill(0)
    fast[12] = 50
    fast[13] = 50
    data.models.items = [
      { ...data.models.items[0], value: "sonnet", label: "sonnet", attempt_count: 100, output_tps: { generating_streaming: outputTPS(100, 100, 28, 22, slow) } },
      { ...data.models.items[0], value: "haiku", label: "haiku", attempt_count: 100, output_tps: { generating_streaming: outputTPS(100, 100, 300, 240, fast) } },
    ]
    render(<AttemptPerformance onRetryProviders={vi.fn()} providers={["claude"]} onSelectProvider={vi.fn()} provider="claude" data={data} isLoading={false} error={null} onRetry={vi.fn()} />)
    fireEvent.click(within(screen.getByLabelText("Performance metric")).getByRole("button", { name: "Output TPS" }))
    const models = screen.getByRole("region", { name: "Models" })
    const sonnet = within(models).getByRole("img", { name: "sonnet: p50 28.0 tok/s, p10 22.0 tok/s" })
    const haiku = within(models).getByRole("img", { name: "haiku: p50 300.0 tok/s, p10 240.0 tok/s" })
    expect(sonnet.querySelectorAll("[data-heatmap-bin]")).toHaveLength(14)
    expect(sonnet.querySelector('[data-heatmap-bin="2"]')).toHaveAttribute("data-share", "0.6")
    expect(sonnet.querySelector('[data-heatmap-bin="2"]')).toHaveAttribute("title", "20–30 tok/s · 60 samples · 60.0%")
    expect(haiku.querySelector('[data-heatmap-bin="13"]')).toHaveAttribute("title", "300+ tok/s · 50 samples · 50.0%")
    // Every band is drawn at the same width regardless of its numeric span.
    expect(sonnet.querySelector<HTMLElement>('[data-heatmap-bin="2"]')?.style.width).toBe(haiku.querySelector<HTMLElement>('[data-heatmap-bin="13"]')?.style.width)
    expect(sonnet.querySelector("[data-band-scale-change]")).toHaveStyle({ left: `${10 / 14 * 100}%` })
    // 300 is the first value of the open band; 240 is 40% through 200–300.
    expect(haiku.querySelector('[data-percentile="p50"]')).toHaveStyle({ left: `${13 / 14 * 100}%` })
    expect(haiku.querySelector('[data-percentile="p10"]')).toHaveStyle({ left: `${12.4 / 14 * 100}%` })
    expect(within(models).queryByRole("link")).not.toBeInTheDocument()
    fireEvent.click(within(models).getByRole("button", { name: "View sample distribution for sonnet" }))
    const details = screen.getByRole("dialog", { name: "sonnet sample distribution" })
    expect(within(details).getByText("100 valid samples · 14 fixed bands")).toBeVisible()
    expect(within(details).getByText("Last band is open-ended.")).toBeVisible()
    const band = within(details).getByText("50–60 tok/s").closest("tr")!
    expect(within(band).getByText("40.0%", { exact: true })).toBeVisible()
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
    data.providers.items[0].output_tps.generating_streaming = outputTPS(10, 5, 42, 42)
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
