import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import type { UsageDashboardSurfaces } from "@/features/usage-intelligence/surfaces"

const liveCapacityLifecycle = vi.hoisted(() => ({ mounts: 0, unmounts: 0 }))

vi.mock("@/components/intelligence/live-capacity-card", async () => {
  const React = await import("react")
  return {
    LiveCapacityCard: class extends React.Component<{ provider: string }> {
      componentDidMount() {
        liveCapacityLifecycle.mounts += 1
      }

      componentWillUnmount() {
        liveCapacityLifecycle.unmounts += 1
      }

      render() {
        return <div>capacity:{this.props.provider}</div>
      }
    },
  }
})
vi.mock("@/components/charts/heatmap", () => ({ Heatmap: () => null }))
vi.mock("@/components/charts/health-grid", () => ({ HealthGrid: () => null }))
vi.mock("@/components/intelligence/request-evidence", () => ({ RequestEvidence: () => null }))
vi.mock("@/components/intelligence/failure-distribution", () => ({ FailureDistribution: () => null }))
vi.mock("@/components/intelligence/model-mappings", () => ({ ModelMappings: () => null }))
vi.mock("@/components/intelligence/attempt-performance", () => ({ AttemptPerformance: () => null }))

import { DashboardFixedOverview } from "./dashboard-fixed-overview"

const surfaces: UsageDashboardSurfaces = {
  core: { status: "empty", data: undefined },
  kpis: { status: "empty" },
  trend: { status: "empty", data: [] },
  leaderboard: { status: "empty", data: [] },
  modelMix: { status: "empty", data: [] },
  insights: { status: "empty", data: [] },
  heatmap: { status: "empty", data: undefined },
  requestHealth: { status: "empty", data: undefined },
}

function overview(provider: string) {
  return (
    <DashboardFixedOverview
      surfaces={surfaces}
      liveCapacityProvider={provider}
      requestEvidenceProvider=""
      isRequestEvidenceLoading={false}
      isRequestEvidenceRefreshing={false}
      requestEvidenceError={null}
      isFailureDistributionLoading={false}
      failureDistributionError={null}
      isModelMappingsLoading={false}
      modelMappingsError={null}
      isAttemptPerformanceLoading={false}
      attemptPerformanceError={null}
      onRetryHeatmap={vi.fn()}
      onRetryRequestHealth={vi.fn()}
      onRetryRequestEvidence={vi.fn()}
      onRetryFailureDistribution={vi.fn()}
      onRetryModelMappings={vi.fn()}
      onRetryAttemptPerformance={vi.fn()}
    />
  )
}

describe("DashboardFixedOverview", () => {
  it("keeps the live-capacity task owner mounted when the provider changes", () => {
    liveCapacityLifecycle.mounts = 0
    liveCapacityLifecycle.unmounts = 0
    const view = render(overview("Codex"))

    expect(screen.getByText("capacity:Codex")).toBeInTheDocument()
    view.rerender(overview("Gemini"))

    expect(screen.getByText("capacity:Gemini")).toBeInTheDocument()
    expect(liveCapacityLifecycle).toEqual({ mounts: 1, unmounts: 0 })
  })
})
