import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { UsageDashboardSurfaces } from "@/features/usage-intelligence/surfaces"
import type { AnalyticsCoreResponse, Insight, ModelDistribution } from "@/types/api"
import { DashboardCharts } from "./dashboard-charts"
import analyticsSummaryFixture from "@/test/contracts/analytics_summary.json"

vi.mock("@/components/charts/model-distribution", () => ({
  ModelDistributionChart: ({ data, measure }: { data: ModelDistribution[]; measure: string }) => (
    <div data-testid="model-distribution-owner">{measure}:{data.map((row) => row.model).join(",")}</div>
  ),
}))

vi.mock("@/components/charts/insight-rail", () => ({
  InsightRail: ({ insights }: { insights: Insight[] }) => (
    <div data-testid="insight-rail-owner">{insights.map((insight) => insight.title).join(",")}</div>
  ),
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

const model = { model: "provider-scoped-model" } as ModelDistribution
const insight = { title: "Pricing incomplete", severity: "amber" } as Insight

describe("DashboardCharts Usage Intelligence fields", () => {
  it("shows selected-window accounting inside the existing trend surface only when the core is ready", () => {
    const props = {
      coreAnalyticsData: analyticsSummaryFixture as AnalyticsCoreResponse,
      effectiveGranularity: "hour" as const,
      trendView: "cost-token" as const,
      onSelectTrendView: vi.fn(),
      leaderboardScope: "api-key" as const,
      onSelectLeaderboardScope: vi.fn(),
      leaderboardSortLabel: "Sort: Cost",
      modelMixMeasure: "tokens" as const,
      modelMixCostStateLabel: "Cost partial, by tokens",
      onRetryCore: vi.fn(),
    }
    const { rerender } = render(<DashboardCharts {...props} surfaces={surfaces()} />)
    expect(screen.getByText("Canonical token composition")).toBeInTheDocument()
    expect(screen.getByText(/0 \/ 3 attempts have valid/)).toBeInTheDocument()
    rerender(<DashboardCharts {...props} surfaces={surfaces({ core: { status: "error", data: undefined, error: new Error("unavailable") } })} />)
    expect(screen.queryByText("Canonical token composition")).not.toBeInTheDocument()
  })

  it("renders the Model Mix owner and conditional attention signals with token fallback labeling", () => {
    render(
      <DashboardCharts
        surfaces={surfaces({
          modelMix: { status: "ready", data: [model] },
          insights: { status: "ready", data: [insight] },
        })}
        effectiveGranularity="hour"
        trendView="cost-token"
        onSelectTrendView={vi.fn()}
        leaderboardScope="api-key"
        onSelectLeaderboardScope={vi.fn()}
        leaderboardSortLabel="Sort: Tokens"
        modelMixMeasure="tokens"
        modelMixCostStateLabel="Cost partial, by tokens"
        onRetryCore={vi.fn()}
      />,
    )

    expect(screen.getByTestId("model-mix-cost-state")).toHaveTextContent("Cost partial, by tokens")
    expect(screen.getByTestId("model-distribution-owner")).toHaveTextContent("tokens:provider-scoped-model")
    expect(screen.getByTestId("insight-rail-owner")).toHaveTextContent("Pricing incomplete")
  })

  it("inherits the core error retry for Model Mix and attention signals", async () => {
    const user = userEvent.setup()
    const onRetryCore = vi.fn()
    render(
      <DashboardCharts
        surfaces={surfaces({
          modelMix: { status: "error", data: [] },
          insights: { status: "error", data: [] },
        })}
        effectiveGranularity="hour"
        trendView="cost-token"
        onSelectTrendView={vi.fn()}
        leaderboardScope="api-key"
        onSelectLeaderboardScope={vi.fn()}
        leaderboardSortLabel="Sort: Cost"
        modelMixMeasure="cost"
        modelMixCostStateLabel="By cost"
        onRetryCore={onRetryCore}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Retry model mix" }))
    await user.click(screen.getByRole("button", { name: "Retry attention signals" }))
    expect(onRetryCore).toHaveBeenCalledTimes(2)
  })
})
