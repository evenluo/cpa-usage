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
  isModelMappingsLoading: boolean
  modelMappingsError: unknown
  attemptPerformanceData?: UsageAttemptPerformance
  isAttemptPerformanceLoading: boolean
  attemptPerformanceError: unknown
  onRetryCore: () => void
  onRetryRequestHealth: () => void
  onRetryRequestEvidence: () => void
  onRetryFailureDistribution: () => void
  onRetryModelMappings: () => void
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
  isModelMappingsLoading,
  modelMappingsError,
  attemptPerformanceData,
  isAttemptPerformanceLoading,
  attemptPerformanceError,
  onRetryCore,
  onRetryRequestHealth,
  onRetryRequestEvidence,
  onRetryFailureDistribution,
  onRetryModelMappings,
  onRetryAttemptPerformance,
}: DashboardAttentionProps) {
  return (
    <>
      {surfaces.insights.status === "loading" ? (
        <Skeleton className="h-24 w-full" />
      ) : surfaces.insights.status === "error" ? (
        <div className="flex min-h-20 items-center justify-between gap-3 rounded-lg border border-dashed border-border px-4 py-3 text-sm text-red-500">
          <span>Failed to load attention signals</span>
          <Button type="button" size="sm" variant="outline" onClick={onRetryCore}>Retry attention signals</Button>
        </div>
      ) : surfaces.insights.status === "ready" ? (
        <InsightRail insights={surfaces.insights.data} />
      ) : null}

      {/* Section divider — fixed 24h diagnostics */}
      <SectionDivider icon={Pin} label="Diagnostics · fixed 24h" />

      {/* Attempt Health + Evidence — 24h fixed */}
      <div className="grid min-w-0 gap-6 xl:grid-cols-[minmax(0,1.6fr)_minmax(320px,0.8fr)]">
        <Card className="flex h-full min-w-0 flex-col overflow-hidden xl:h-[300px]">
          <CardHeader className="flex flex-col items-start justify-between gap-4 pb-2 sm:flex-row">
            <div>
              <CardTitle>Attempt Health</CardTitle>
              <CardDescription>Success rate · 3 min</CardDescription>
            </div>
            {surfaces.requestHealth.status !== "error" && surfaces.requestHealth.refreshError ? (
              <Button type="button" size="sm" variant="outline" onClick={onRetryRequestHealth}>Retry refresh</Button>
            ) : null}
          </CardHeader>
          <CardContent className="min-h-0 min-w-0 flex-1">
            {surfaces.requestHealth.status === "loading" ? (
              <Skeleton className="h-[180px] w-full" />
            ) : surfaces.requestHealth.status === "error" ? (
              <div className="flex h-[180px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
                <span>Failed to load attempt health</span>
                <Button type="button" size="sm" variant="outline" onClick={onRetryRequestHealth}>Retry attempt health</Button>
              </div>
            ) : surfaces.requestHealth.status === "ready" ? (
              <HealthGrid data={surfaces.requestHealth.data} />
            ) : (
              <div className="flex h-[180px] items-center justify-center text-sm text-muted-foreground">
                No health data
              </div>
            )}
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

      <FailureDistribution
        provider={requestEvidenceProvider}
        data={failureDistributionData}
        isLoading={isFailureDistributionLoading}
        error={failureDistributionError}
        onRetry={onRetryFailureDistribution}
      />

      <AttemptPerformance
        provider={requestEvidenceProvider}
        data={attemptPerformanceData}
        isLoading={isAttemptPerformanceLoading}
        error={attemptPerformanceError}
        onRetry={onRetryAttemptPerformance}
      />

      <ModelMappings
        data={modelMappingsData}
        isLoading={isModelMappingsLoading}
        error={modelMappingsError}
        onRetry={onRetryModelMappings}
      />
    </>
  )
}
