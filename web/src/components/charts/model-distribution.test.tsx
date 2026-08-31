import type { ReactNode } from "react"
import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { ModelDistribution } from "@/types/api"
import { ModelDistributionChart } from "./model-distribution"

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  PieChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Pie: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Cell: () => null,
}))

afterEach(cleanup)

function model(index: number): ModelDistribution {
  return {
    model: `model-${index}`,
    provider: `provider-${index}`,
    total_cost: index,
    total_tokens: index * 1_000,
    input_tokens: index * 500,
    output_tokens: index * 400,
    reasoning_tokens: index * 100,
    cached_tokens: 0,
    cache_read_share: 0,
    cache_read_share_state: "no_cache_data",
    request_count: index * 2,
    success_count: index * 2,
    failure_count: 0,
    success_rate: 100,
    total_latency_ms: 0,
    latency_sample_count: 0,
    average_latency_ms: 0,
    cost_available: true,
    cost_status: "available",
  }
}

describe("ModelDistributionChart", () => {
  it("uses a leading-model composition and groups the long tail on wide data sets", () => {
    render(<ModelDistributionChart data={[1, 2, 3, 4, 5, 6, 7].map(model)} measure="cost" />)

    expect(screen.getByText("7", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("models", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("model-7", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("25.0%", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("Other models", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("2 additional models", { exact: true })).toBeInTheDocument()
    expect(screen.queryByText("model-1", { exact: true })).not.toBeInTheDocument()
  })

  it("keeps token share authoritative while showing cost as supporting context", () => {
    render(<ModelDistributionChart data={[model(1), model(2)]} measure="tokens" />)

    expect(screen.getByText("Token mix", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("model-2", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("66.7%", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("$2.00", { exact: true })).toBeInTheDocument()
  })
})
