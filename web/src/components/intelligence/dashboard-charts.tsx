import { Clock } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { KeyLeaderboard } from "@/components/charts/key-leaderboard"
import { TrendChart } from "@/components/charts/trend-chart"
import type { UsageDashboardSurfaces } from "@/features/usage-intelligence/surfaces"
import type { LeaderboardScope, TrendView } from "@/features/usage-intelligence/view-model"
import type { AnalyticsCoreResponse, TimeGranularity } from "@/types/api"
import { cn } from "@/lib/utils"

interface DashboardChartsProps {
  surfaces: UsageDashboardSurfaces
  coreAnalyticsData?: AnalyticsCoreResponse
  effectiveGranularity: TimeGranularity
  trendView: TrendView
  onSelectTrendView: (view: TrendView) => void
  leaderboardScope: LeaderboardScope
  onSelectLeaderboardScope: (scope: LeaderboardScope) => void
  leaderboardSortLabel: string
}

export function DashboardCharts({
  surfaces,
  coreAnalyticsData,
  effectiveGranularity,
  trendView,
  onSelectTrendView,
  leaderboardScope,
  onSelectLeaderboardScope,
  leaderboardSortLabel,
}: DashboardChartsProps) {
  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_400px]">
      {/* Trend Chart */}
      <Card>
        <CardHeader className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:flex-wrap">
          <div>
            <CardTitle className="flex items-center gap-2">
              Trend Workbench
              <Clock className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Affected by time range and granularity" />
            </CardTitle>
            <CardDescription>
              {trendView === "cost-token" && "Cost as filled area, tokens as dotted overlay"}
              {trendView === "requests-token" && "Requests as filled area, tokens as dotted overlay"}
              {trendView === "tokens" && "Total, input, output, reasoning, and cached tokens"}
            </CardDescription>
          </div>
          <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
            {[
              { value: "cost-token", label: "Cost" },
              { value: "requests-token", label: "Requests" },
              { value: "tokens", label: "Tokens" },
            ].map((item) => (
              <button
                key={item.value}
                onClick={() => onSelectTrendView(item.value as TrendView)}
                aria-label={`Trend view: ${item.label}`}
                aria-pressed={trendView === item.value}
                className={cn(
                  "shrink-0 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  trendView === item.value
                    ? "bg-terracotta-500 text-white"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                )}
              >
                {item.label}
              </button>
            ))}
          </div>
        </CardHeader>
        <CardContent>
          {surfaces.trend.status === "loading" ? (
            <Skeleton className="h-[260px] w-full" />
          ) : surfaces.trend.status === "error" ? (
            <div className="flex h-[260px] items-center justify-center text-sm text-red-500">
              Failed to load trend data
            </div>
          ) : (
            <div className="h-[260px]">
              <TrendChart data={surfaces.trend.data} granularity={coreAnalyticsData?.granularity ?? effectiveGranularity} mode={trendView} />
            </div>
          )}
        </CardContent>
      </Card>

      {/* Key Leaderboard */}
      <Card>
        <CardHeader className="flex flex-col items-start justify-between gap-3 pb-2 sm:flex-row sm:flex-wrap">
          <div>
            <CardTitle className="flex items-center gap-2">
              Key Leaderboard
              <Clock className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Affected by time range and granularity" />
            </CardTitle>
            <CardDescription>
              {leaderboardScope === "api-key" ? "Top raw API keys" : "Top account keys"}
            </CardDescription>
          </div>
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:justify-end">
            <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
              {[
                { value: "api-key", label: "API Keys" },
                { value: "account", label: "Accounts" },
              ].map((item) => (
                <button
                  key={item.value}
                  onClick={() => onSelectLeaderboardScope(item.value as LeaderboardScope)}
                  aria-label={`Leaderboard scope: ${item.label}`}
                  aria-pressed={leaderboardScope === item.value}
                  className={cn(
                    "shrink-0 rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
                    leaderboardScope === item.value
                      ? "bg-terracotta-500 text-white"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
            <Badge variant="terracotta">{leaderboardSortLabel}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          {surfaces.leaderboard.status === "loading" ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : (
            <KeyLeaderboard data={surfaces.leaderboard.data} />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
