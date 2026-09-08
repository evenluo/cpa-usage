import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import type { UsageDashboardSurfaces } from "@/features/usage-intelligence/surfaces"
import type { Insight } from "@/types/api"

vi.mock("@/components/charts/health-grid", () => ({ HealthGrid: () => null }))
vi.mock("@/components/intelligence/request-evidence", () => ({ RequestEvidence: () => null }))
vi.mock("@/components/intelligence/failure-distribution", () => ({ FailureDistribution: () => <div>Failure detail owner</div> }))
vi.mock("@/components/intelligence/model-mappings", () => ({ ModelMappings: () => null }))
vi.mock("@/components/intelligence/attempt-performance", () => ({ AttemptPerformance: () => <div>Performance comparison owner</div> }))

import { DashboardAttention } from "./dashboard-attention"

function makeSurfaces(insights: UsageDashboardSurfaces["insights"]): UsageDashboardSurfaces {
  return {
    core: { status: "empty", data: undefined },
    kpis: { status: "empty" },
    trend: { status: "empty", data: [] },
    leaderboard: { status: "empty", data: [] },
    modelMix: { status: "empty", data: [] },
    insights,
    heatmap: { status: "empty", data: undefined },
    requestHealth: { status: "empty", data: undefined },
  }
}

function attention(surfaces: UsageDashboardSurfaces, onRetryCore = vi.fn()) {
  return {
    onRetryCore,
    element: (
      <DashboardAttention
        surfaces={surfaces}
        requestEvidenceProvider=""
        isRequestEvidenceLoading={false}
        isRequestEvidenceRefreshing={false}
        requestEvidenceError={null}
        isFailureDistributionLoading={false}
        failureDistributionError={null}
        isModelMappingsSummaryLoading={false}
        modelMappingsSummaryError={null}
        isModelMappingDetailsLoading={false}
        modelMappingDetailsError={null}
        onModelMappingsExpandedChange={vi.fn()}
        performanceProvidersError={null}
        onRetryPerformanceProviders={vi.fn()}
        attemptPerformanceProvider="claude"
        attemptPerformanceProviders={["claude"]}
        onSelectPerformanceProvider={vi.fn()}
        isAttemptPerformanceLoading={false}
        attemptPerformanceError={null}
        onRetryCore={onRetryCore}
        onRetryRequestHealth={vi.fn()}
        onRetryRequestEvidence={vi.fn()}
        onRetryFailureDistribution={vi.fn()}
        onRetryModelMappingsSummary={vi.fn()}
        onRetryModelMappingDetails={vi.fn()}
        onRetryAttemptPerformance={vi.fn()}
      />
    ),
  }
}

const sampleInsight: Insight = {
  type: "failure_concentration",
  severity: "amber",
  title: "Failures cluster",
  detail: "5xx on one account",
  subject: "account",
  metric_label: "failures",
  metric_value: 3,
  count: 3,
  cost_status: "available",
}

describe("DashboardAttention", () => {
  it("puts performance first and failure detail inside Attempt Health", () => {
    render(attention(makeSurfaces({ status: "empty", data: [] })).element)
    const performance = screen.getByText("Performance comparison owner")
    const health = screen.getByRole("heading", { name: "Attempt Health" })
    expect(performance.compareDocumentPosition(health) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(health.closest(".rounded-xl")).toContainElement(screen.getByText("Failure detail owner"))
  })

  it("renders the insight rail when attention signals are ready", () => {
    render(attention(makeSurfaces({ status: "ready", data: [sampleInsight] })).element)

    expect(screen.getByText("Failures cluster")).toBeInTheDocument()
  })

  it("wires the insight error retry to the core analytics retry", async () => {
    const onRetryCore = vi.fn()
    render(attention(makeSurfaces({ status: "error", data: [] }), onRetryCore).element)

    await userEvent.click(screen.getByRole("button", { name: "Retry attention signals" }))

    expect(onRetryCore).toHaveBeenCalledTimes(1)
  })
})
