import { Clock } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { KeyLeaderboard } from "@/components/charts/key-leaderboard"
import { InsightRail } from "@/components/charts/insight-rail"
import { ModelDistributionChart } from "@/components/charts/model-distribution"
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
  modelMixMeasure: "cost" | "tokens"
  modelMixCostStateLabel: string
  onRetryCore: () => void
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
  modelMixMeasure,
  modelMixCostStateLabel,
  onRetryCore,
}: DashboardChartsProps) {
  return (
    <div className="space-y-6">
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
              {trendView === "requests-token" && "Attempts as filled area, tokens as dotted overlay"}
              {trendView === "tokens" && "Total, input, output, reasoning, and cached tokens"}
            </CardDescription>
          </div>
          <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
            {[
              { value: "cost-token", label: "Cost" },
              { value: "requests-token", label: "Attempts" },
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
            <div className="flex h-[260px] flex-col items-center justify-center gap-3 text-sm text-red-500">
              <span>Failed to load trend data</span>
              <Button type="button" size="sm" variant="outline" onClick={onRetryCore}>Retry trend data</Button>
            </div>
          ) : surfaces.trend.status === "empty" ? (
            <div className="flex h-[260px] items-center justify-center text-sm text-muted-foreground">No trend data</div>
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
          ) : surfaces.leaderboard.status === "error" ? (
            <div className="flex min-h-32 flex-col items-center justify-center gap-3 text-sm text-red-500">
              <span>Failed to load key leaderboard</span>
              <Button type="button" size="sm" variant="outline" onClick={onRetryCore}>Retry leaderboard</Button>
            </div>
          ) : surfaces.leaderboard.status === "empty" ? (
            <div className="flex min-h-32 items-center justify-center text-sm text-muted-foreground">No keys in this window</div>
          ) : (
            <KeyLeaderboard data={surfaces.leaderboard.data} />
          )}
        </CardContent>
      </Card>
      </div>

      {surfaces.insights.status === "loading" ? (
        <Skeleton className="h-24 w-full" />
      ) : surfaces.insights.status === "error" ? (
        <div className="flex min-h-20 items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50/60 px-4 py-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/20 dark:text-amber-200">
          <span>Failed to load attention signals</span>
          <Button type="button" size="sm" variant="outline" onClick={onRetryCore}>Retry attention signals</Button>
        </div>
      ) : surfaces.insights.status === "ready" ? (
        <InsightRail insights={surfaces.insights.data} />
      ) : null}

      <Card>
        <CardHeader className="flex flex-col items-start justify-between gap-3 sm:flex-row sm:items-center">
          <div>
            <CardTitle>Model Mix</CardTitle>
            <CardDescription>Share within the model breakdown returned for the selected window</CardDescription>
          </div>
          <Badge
            variant={modelMixMeasure === "cost" ? "terracotta" : "outline"}
            data-testid="model-mix-cost-state"
          >
            {modelMixCostStateLabel}
          </Badge>
        </CardHeader>
        <CardContent>
          {surfaces.modelMix.status === "loading" ? (
            <Skeleton className="h-[300px] w-full" />
          ) : surfaces.modelMix.status === "error" ? (
            <div className="flex min-h-[300px] flex-col items-center justify-center gap-3 text-sm text-red-500">
              <span>Failed to load model mix</span>
              <Button type="button" size="sm" variant="outline" onClick={onRetryCore}>Retry model mix</Button>
            </div>
          ) : surfaces.modelMix.status === "empty" ? (
            <div className="flex min-h-[300px] items-center justify-center text-sm text-muted-foreground">No model usage in this window</div>
          ) : (
            <ModelDistributionChart data={surfaces.modelMix.data} measure={modelMixMeasure} />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
