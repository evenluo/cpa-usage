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
    cache_read_tokens: 0,
    cache_read_share: 0,
    cache_read_coverage: 0,
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
  it("scopes composition and long-tail labels to the returned model set", () => {
    render(<ModelDistributionChart data={[1, 2, 3, 4, 5, 6, 7].map(model)} measure="cost" />)

    expect(screen.getByText("7", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("models shown", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("Shown cost mix", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("Leading shown model", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("model-7", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("25.0%", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("Other shown models", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("2 additional shown models", { exact: true })).toBeInTheDocument()
    expect(screen.queryByText("model-1", { exact: true })).not.toBeInTheDocument()
  })

  it("labels token share as scoped to the returned models while showing cost context", () => {
    render(<ModelDistributionChart data={[model(1), model(2)]} measure="tokens" />)

    expect(screen.getByText("Shown token mix", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("model-2", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("66.7%", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("$2.00", { exact: true })).toBeInTheDocument()
  })

  it("does not format an unavailable model cost in the token presentation", () => {
    render(
      <ModelDistributionChart
        data={[{ ...model(1), total_cost: 0, cost_available: false, cost_status: "partial" }]}
        measure="tokens"
      />,
    )

    expect(screen.getByText("Cost n/a", { exact: true })).toBeInTheDocument()
    expect(screen.queryByText("$0.00", { exact: true })).not.toBeInTheDocument()
  })

  it("marks Other cost unavailable when any aggregated model is incomplete", () => {
    render(
      <ModelDistributionChart
        data={[
          ...[3, 4, 5, 6, 7].map(model),
          { ...model(2), total_cost: 0 },
          { ...model(1), total_cost: 0, cost_available: false, cost_status: "unavailable" },
        ]}
        measure="tokens"
      />,
    )

    expect(screen.getByText("Other shown models", { exact: true })).toBeInTheDocument()
    expect(screen.getByText("Cost n/a", { exact: true })).toBeInTheDocument()
    expect(screen.queryByText("$0.00", { exact: true })).not.toBeInTheDocument()
  })

  it("shows an explicit zero-value state instead of naming a leading model", () => {
    render(
      <ModelDistributionChart
        data={[model(1), model(2)].map((row) => ({ ...row, total_cost: 0 }))}
        measure="cost"
      />,
    )

    expect(screen.getByText("No cost recorded for shown models", { exact: true })).toBeInTheDocument()
    expect(screen.queryByText("Leading shown model", { exact: true })).not.toBeInTheDocument()
    expect(screen.queryByText("0.0%", { exact: true })).not.toBeInTheDocument()
  })
})
