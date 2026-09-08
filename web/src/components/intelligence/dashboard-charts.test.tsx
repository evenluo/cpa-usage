import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageDashboardSurfaces } from "@/features/usage-intelligence/surfaces"
import type { AnalyticsCoreResponse, ModelDistribution } from "@/types/api"
import { DashboardActivity, DashboardCharts } from "./dashboard-charts"

vi.mock("@/components/charts/model-distribution", () => ({
  ModelDistributionChart: ({ data, measure }: { data: ModelDistribution[]; measure: string }) => (
    <div data-testid="model-distribution-owner">{measure}:{data.map((row) => row.model).join(",")}</div>
  ),
}))

vi.mock("@/components/charts/heatmap", () => ({
  Heatmap: () => <div data-testid="heatmap-owner" />,
}))

afterEach(cleanup)

function surfaces(overrides: Partial<UsageDashboardSurfaces> = {}): UsageDashboardSurfaces {
  return {
    core: { status: "ready", data: {} as AnalyticsCoreResponse },
    kpis: { status: "ready" },
    trend: { status: "empty", data: [] },
    leaderboard: { status: "empty", data: [] },
    modelMix: { status: "empty", data: [] },
    insights: { status: "empty", data: [] },
    heatmap: { status: "empty", data: undefined },
    requestHealth: { status: "empty", data: undefined },
    ...overrides,
  }
}

function makeProps(overrides: Record<string, unknown> = {}) {
  return {
    effectiveGranularity: "hour" as const,
    trendView: "cost-token" as const,
    onSelectTrendView: vi.fn(),
    leaderboardScope: "api-key" as const,
    onSelectLeaderboardScope: vi.fn(),
    leaderboardSortLabel: "Sort: Cost",
    modelMixMeasure: "tokens" as const,
    modelMixCostStateLabel: "Local estimate incomplete, by tokens",
    onRetryCore: vi.fn(),
    ...overrides,
  }
}

const model = { model: "provider-scoped-model" } as ModelDistribution

describe("DashboardCharts Usage Intelligence fields", () => {
  it("renders the Model Mix owner with token fallback labeling", () => {
    render(
      <DashboardCharts
        {...makeProps()}
        surfaces={surfaces({ modelMix: { status: "ready", data: [model] } })}
      />,
    )

    expect(screen.getByTestId("model-mix-cost-state")).toHaveTextContent("Local estimate incomplete, by tokens")
    expect(screen.getByTestId("model-distribution-owner")).toHaveTextContent("tokens:provider-scoped-model")
  })

  it("inherits the core error retry for Model Mix", async () => {
    const user = userEvent.setup()
    const onRetryCore = vi.fn()
    render(
      <DashboardCharts
        {...makeProps({ onRetryCore, modelMixMeasure: "cost", modelMixCostStateLabel: "By cost" })}
        surfaces={surfaces({ modelMix: { status: "error", data: [] } })}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Retry model mix" }))
    expect(onRetryCore).toHaveBeenCalledTimes(1)
  })

  it("owns the fixed 30d heatmap with its own retry wiring", async () => {
    const user = userEvent.setup()
    const onRetryHeatmap = vi.fn()
    const { rerender } = render(
      <DashboardActivity
        onRetryHeatmap={onRetryHeatmap}
        surfaces={surfaces({ heatmap: { status: "ready", data: { rows: [] } as never } })}
      />,
    )
    expect(screen.getByTestId("heatmap-owner")).toBeInTheDocument()

    rerender(
      <DashboardActivity
        onRetryHeatmap={onRetryHeatmap}
        surfaces={surfaces({ heatmap: { status: "error", data: undefined, error: new Error("unavailable") } })}
      />,
    )
    await user.click(screen.getByRole("button", { name: "Retry heatmap" }))
    expect(onRetryHeatmap).toHaveBeenCalledTimes(1)
  })
})
