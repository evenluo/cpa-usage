import { Pin } from "lucide-react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { HealthGrid } from "@/components/charts/health-grid"
import { InsightRail } from "@/components/charts/insight-rail"
import { RequestEvidence } from "@/components/intelligence/request-evidence"
import { FailureDistribution } from "@/components/intelligence/failure-distribution"
import { ModelMappings } from "@/components/intelligence/model-mappings"
import { AttemptPerformance } from "@/components/intelligence/attempt-performance"
import { SectionDivider } from "@/components/intelligence/section-divider"
import type { UsageDashboardSurfaces } from "@/features/usage-intelligence/surfaces"
import type { UsageModelMappingSummary } from "@/features/usage-intelligence/model-mapping-summary"
import type { UsageAttemptPerformance, UsageEventsPage, UsageFailureDistribution, UsageModelMappingDistribution } from "@/types/api"

interface DashboardAttentionProps {
  surfaces: UsageDashboardSurfaces
  requestEvidenceProvider: string
  requestEvidenceData?: UsageEventsPage
  isRequestEvidenceLoading: boolean
  isRequestEvidenceRefreshing: boolean
  requestEvidenceError: unknown
  failureDistributionData?: UsageFailureDistribution
  isFailureDistributionLoading: boolean
  failureDistributionError: unknown
  modelMappingsData?: UsageModelMappingDistribution
  modelMappingsSummaryData?: UsageModelMappingSummary
  isModelMappingsSummaryLoading: boolean
  modelMappingsSummaryError: unknown
  isModelMappingDetailsLoading: boolean
  modelMappingDetailsError: unknown
  onModelMappingsExpandedChange: (expanded: boolean) => void
  attemptPerformanceProvider: string
  attemptPerformanceProviders: string[]
  onSelectPerformanceProvider: (provider: string) => void
  performanceProvidersError: unknown
  onRetryPerformanceProviders: () => void
  attemptPerformanceData?: UsageAttemptPerformance
  isAttemptPerformanceLoading: boolean
  attemptPerformanceError: unknown
  onRetryCore: () => void
  onRetryRequestHealth: () => void
  onRetryRequestEvidence: () => void
  onRetryFailureDistribution: () => void
  onRetryModelMappingsSummary: () => void
  onRetryModelMappingDetails: () => void
  onRetryAttemptPerformance: () => void
}

export function DashboardAttention({
  surfaces,
  requestEvidenceProvider,
  requestEvidenceData,
  isRequestEvidenceLoading,
  isRequestEvidenceRefreshing,
  requestEvidenceError,
  failureDistributionData,
  isFailureDistributionLoading,
  failureDistributionError,
  modelMappingsData,
  modelMappingsSummaryData,
  isModelMappingsSummaryLoading,
  modelMappingsSummaryError,
  isModelMappingDetailsLoading,
  modelMappingDetailsError,
  onModelMappingsExpandedChange,
  attemptPerformanceProvider,
  attemptPerformanceProviders,
  onSelectPerformanceProvider,
  performanceProvidersError,
  onRetryPerformanceProviders,
  attemptPerformanceData,
  isAttemptPerformanceLoading,
  attemptPerformanceError,
  onRetryCore,
  onRetryRequestHealth,
  onRetryRequestEvidence,
  onRetryFailureDistribution,
  onRetryModelMappingsSummary,
  onRetryModelMappingDetails,
  onRetryAttemptPerformance,
}: DashboardAttentionProps) {
  return (
    <>
      {surfaces.insights.status === "loading" ? (
        <Skeleton className="h-24 w-full" />
      ) : surfaces.insights.status === "error" ? (
        <div className="flex min-h-20 items-center justify-between gap-3 rounded-lg border border-dashed border-border px-4 py-3 text-sm text-red-500">
          <span>Couldn't load warnings</span>
          <Button type="button" size="sm" variant="outline" onClick={onRetryCore}>Retry</Button>
        </div>
      ) : surfaces.insights.status === "ready" ? (
        <InsightRail insights={surfaces.insights.data} />
      ) : null}

      {/* Section divider — fixed 24h diagnostics */}
      <SectionDivider icon={Pin} label="Performance & health" />

      <AttemptPerformance
        provider={attemptPerformanceProvider}
        providers={attemptPerformanceProviders}
        providersError={performanceProvidersError}
        onRetryProviders={onRetryPerformanceProviders}
        onSelectProvider={onSelectPerformanceProvider}
        data={attemptPerformanceData}
        isLoading={isAttemptPerformanceLoading}
        error={attemptPerformanceError}
        onRetry={onRetryAttemptPerformance}
      />

      {/* Attempt Health + Evidence — 24h fixed */}
      <div className="grid min-w-0 items-start gap-6 xl:grid-cols-[minmax(0,1.6fr)_minmax(320px,0.8fr)]">
        <Card className="min-w-0 overflow-hidden">
          <CardHeader className="flex flex-col items-start justify-between gap-4 pb-2 sm:flex-row">
            <div>
              <CardTitle>Attempt Health</CardTitle>
              <CardDescription>Last 24h · Success rate every 3 min</CardDescription>
            </div>
            {surfaces.requestHealth.status !== "error" && surfaces.requestHealth.refreshError ? (
              <Button type="button" size="sm" variant="outline" onClick={onRetryRequestHealth}>Retry</Button>
            ) : null}
          </CardHeader>
          <CardContent className="min-h-0 min-w-0 flex-1">
            {surfaces.requestHealth.status === "loading" ? (
              <Skeleton className="h-[180px] w-full" />
            ) : surfaces.requestHealth.status === "error" ? (
              <div className="flex h-[180px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
                <span>Couldn't load attempt health</span>
                <Button type="button" size="sm" variant="outline" onClick={onRetryRequestHealth}>Retry</Button>
              </div>
            ) : surfaces.requestHealth.status === "ready" ? (
              <HealthGrid data={surfaces.requestHealth.data} />
            ) : (
              <div className="flex h-[180px] items-center justify-center text-sm text-muted-foreground">
                No health data
              </div>
            )}

            <FailureDistribution
              provider={requestEvidenceProvider}
              data={failureDistributionData}
              isLoading={isFailureDistributionLoading}
              error={failureDistributionError}
              onRetry={onRetryFailureDistribution}
            />
          </CardContent>
        </Card>

        <RequestEvidence
          provider={requestEvidenceProvider}
          data={requestEvidenceData}
          isLoading={isRequestEvidenceLoading}
          isRefreshing={isRequestEvidenceRefreshing}
          error={requestEvidenceError}
          onRetry={onRetryRequestEvidence}
        />
      </div>

      <ModelMappings
        key={requestEvidenceProvider}
        summary={modelMappingsSummaryData}
        data={modelMappingsData}
        isSummaryLoading={isModelMappingsSummaryLoading}
        summaryError={modelMappingsSummaryError}
        isDetailsLoading={isModelMappingDetailsLoading}
        detailsError={modelMappingDetailsError}
        onExpandedChange={onModelMappingsExpandedChange}
        onRetrySummary={onRetryModelMappingsSummary}
        onRetryDetails={onRetryModelMappingDetails}
      />
    </>
  )
}
