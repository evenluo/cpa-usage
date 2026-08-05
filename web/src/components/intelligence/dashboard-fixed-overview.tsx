import { Pin } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Heatmap } from "@/components/charts/heatmap"
import { HealthGrid } from "@/components/charts/health-grid"
import { LiveCapacityCard } from "@/components/intelligence/live-capacity-card"
import { RequestEvidence } from "@/components/intelligence/request-evidence"
import type { UsageDashboardSurfaces } from "@/features/usage-intelligence/surfaces"
import type { UsageEventsPage } from "@/types/api"

interface DashboardFixedOverviewProps {
  surfaces: UsageDashboardSurfaces
  liveCapacityProvider: string
  requestEvidenceData?: UsageEventsPage
  isRequestEvidenceLoading: boolean
  isRequestEvidenceRefreshing: boolean
  requestEvidenceError: unknown
}

export function DashboardFixedOverview({
  surfaces,
  liveCapacityProvider,
  requestEvidenceData,
  isRequestEvidenceLoading,
  isRequestEvidenceRefreshing,
  requestEvidenceError,
}: DashboardFixedOverviewProps) {
  return (
    <>
      {/* Divider — Fixed overview */}
      <div className="flex items-center gap-3">
        <div className="h-px flex-1 bg-border" />
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <Pin className="h-3 w-3" />
          Fixed overview
        </span>
        <div className="h-px flex-1 bg-border" />
      </div>

      <LiveCapacityCard provider={liveCapacityProvider} />

      {/* Activity Heatmap — 30d fixed */}
      <Card>
        <CardHeader className="flex flex-col items-start justify-between gap-3 pb-2 sm:flex-row">
          <div>
            <CardTitle className="flex items-center gap-2">
              Activity Heatmap
              <Pin className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Fixed 30-day view" />
            </CardTitle>
            <CardDescription>Hourly usage density across days</CardDescription>
          </div>
          <Badge variant="terracotta">30d fixed</Badge>
        </CardHeader>
        <CardContent>
          {surfaces.heatmap.status === "loading" ? (
            <Skeleton className="h-[260px] w-full" />
          ) : surfaces.heatmap.status === "error" ? (
            <div className="flex h-[260px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
              Failed to load activity heatmap
            </div>
          ) : surfaces.heatmap.status === "ready" ? (
            <Heatmap data={surfaces.heatmap.data} />
          ) : (
            <div className="flex h-[260px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
              No heatmap data
            </div>
          )}
        </CardContent>
      </Card>

      {/* Request Health + Evidence — 24h fixed */}
      <div className="grid min-w-0 gap-6 xl:grid-cols-[minmax(0,1.6fr)_minmax(320px,0.8fr)]">
        <Card className="flex h-full min-w-0 flex-col overflow-hidden xl:h-[300px]">
          <CardHeader className="flex flex-col items-start justify-between gap-4 pb-2 sm:flex-row">
            <div>
              <CardTitle className="flex items-center gap-2">
                Request Health
                <Pin className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Fixed 24-hour view" />
              </CardTitle>
              <CardDescription>Success rate per 3-minute bucket</CardDescription>
            </div>
            <Badge variant="green">24h fixed</Badge>
          </CardHeader>
          <CardContent className="min-h-0 min-w-0 flex-1">
            {surfaces.requestHealth.status === "loading" ? (
              <Skeleton className="h-[180px] w-full" />
            ) : surfaces.requestHealth.status === "error" ? (
              <div className="flex h-[180px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
                Failed to load request health
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
          data={requestEvidenceData}
          isLoading={isRequestEvidenceLoading}
          isRefreshing={isRequestEvidenceRefreshing}
          error={requestEvidenceError}
        />
      </div>
    </>
  )
}
