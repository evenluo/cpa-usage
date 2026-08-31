import { createLazyFileRoute } from "@tanstack/react-router"
import { KpiCard } from "@/components/intelligence/kpi-card"
import { DashboardCharts } from "@/components/intelligence/dashboard-charts"
import { DashboardControls } from "@/components/intelligence/dashboard-controls"
import { DashboardFixedOverview } from "@/components/intelligence/dashboard-fixed-overview"
import { DashboardCoreEmptyState } from "@/components/intelligence/dashboard-core-empty-state"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { formatCost, formatCompact, formatPercent } from "@/lib/format"
import { useUsageDashboard } from "@/features/usage-intelligence/use-usage-dashboard"

export const Route = createLazyFileRoute("/")({
  component: DashboardPage,
})

function DashboardPage() {
  const dashboard = useUsageDashboard()
  const { summary } = dashboard.coreAnalyticsData ?? {}
  const { surfaces, viewModel, loadPlan } = dashboard
  const { providerOptions, leaderboardSortLabel, cacheReadShareCaption, kpiData } = viewModel

  return (
    <div className="animate-slide-up mx-auto max-w-7xl space-y-6">
      <DashboardControls
        range={dashboard.range}
        onSelectRange={dashboard.selectRange}
        onSelectGranularity={dashboard.setGranularity}
        effectiveGranularity={dashboard.effectiveGranularity}
        provider={dashboard.provider}
        onSelectProvider={dashboard.setProvider}
        providerOptions={providerOptions}
      />

      {surfaces.core.status === "error" ? (
        <Card>
          <CardContent className="flex min-h-32 flex-col items-center justify-center gap-3 text-center">
            <p className="text-sm text-red-600">Failed to load selected-window usage</p>
            <Button type="button" size="sm" variant="outline" onClick={dashboard.retryCore}>Retry usage summary</Button>
          </CardContent>
        </Card>
      ) : surfaces.core.status === "empty" ? (
        <DashboardCoreEmptyState refreshError={surfaces.core.refreshError} onRetry={dashboard.retryCore} />
      ) : (
        <>
          {surfaces.core.status === "ready" && surfaces.core.refreshError ? (
            <div className="flex items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
              <span>Usage refresh failed; showing the last complete result.</span>
              <Button type="button" size="sm" variant="outline" onClick={dashboard.retryCore}>Retry</Button>
            </div>
          ) : null}
          {/* KPI Cards — 5 compact cards */}
          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <KpiCard
          label="Cost"
          rawValue={summary?.total_cost}
          formatter={formatCost}
          valueDecimals={4}
          caption={summary?.cost_status}
          sparkline={kpiData?.cost}
          isLoading={surfaces.kpis.status === "loading"}
          tone="terracotta"
        />
        <KpiCard
          label="Tokens"
          rawValue={summary?.total_tokens}
          formatter={(n) => formatCompact(n, 2)}
          sparkline={kpiData?.tokens}
          isLoading={surfaces.kpis.status === "loading"}
          tone="blue"
        />
        <KpiCard
          label="Requests"
          rawValue={summary?.request_count}
          formatter={(n) => n.toLocaleString("en")}
          sparkline={kpiData?.requests}
          isLoading={surfaces.kpis.status === "loading"}
          tone="violet"
        />
        <KpiCard
          label="Success"
          rawValue={summary?.success_rate}
          formatter={formatPercent}
          valueDecimals={1}
          caption={`${summary?.failure_count ?? 0} failed`}
          sparkline={kpiData?.successRate}
          isLoading={surfaces.kpis.status === "loading"}
          tone="green"
        />
        <KpiCard
          label="Cache"
          rawValue={summary?.cache_read_share}
          formatter={formatPercent}
          valueDecimals={1}
          caption={cacheReadShareCaption}
          isLoading={surfaces.kpis.status === "loading"}
          tone="amber"
        />
          </div>
        </>
      )}

      <DashboardCharts
        surfaces={surfaces}
        coreAnalyticsData={dashboard.coreAnalyticsData}
        effectiveGranularity={dashboard.effectiveGranularity}
        trendView={dashboard.trendView}
        onSelectTrendView={dashboard.setTrendView}
        leaderboardScope={dashboard.leaderboardScope}
        onSelectLeaderboardScope={dashboard.setLeaderboardScope}
        leaderboardSortLabel={leaderboardSortLabel}
        modelMixMeasure={viewModel.modelMixMeasure}
        modelMixCostStateLabel={viewModel.modelMixCostStateLabel}
        onRetryCore={dashboard.retryCore}
      />

      <DashboardFixedOverview
        surfaces={surfaces}
        liveCapacityProvider={loadPlan.fixedWindow.liveCapacity.provider}
        requestEvidenceProvider={loadPlan.fixedWindow.requestEvidence.provider}
        requestEvidenceData={dashboard.requestEvidenceData}
        isRequestEvidenceLoading={dashboard.isRequestEvidenceLoading}
        isRequestEvidenceRefreshing={dashboard.isRequestEvidenceRefreshing}
        requestEvidenceError={dashboard.requestEvidenceError}
        onRetryHeatmap={dashboard.retryHeatmap}
        onRetryRequestHealth={dashboard.retryRequestHealth}
        onRetryRequestEvidence={dashboard.retryRequestEvidence}
      />
    </div>
  )
}
