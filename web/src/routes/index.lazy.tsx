import { createLazyFileRoute } from "@tanstack/react-router"
import { useEffect, useMemo, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"

import { Skeleton } from "@/components/ui/skeleton"
import { useAnalytics } from "@/hooks/useAnalytics"
import { useCountUp } from "@/hooks/useCountUp"
import { Sparkline } from "@/components/charts/sparkline"
import { TrendChart } from "@/components/charts/trend-chart"
import { KeyLeaderboard } from "@/components/charts/key-leaderboard"
import { Heatmap } from "@/components/charts/heatmap"
import { HealthGrid } from "@/components/charts/health-grid"
import { RequestEvidence } from "@/components/intelligence/request-evidence"
import { useUsageOverview } from "@/hooks/useUsageOverview"
import { formatCost, formatCompact, formatComparison } from "@/lib/format"
import {
  buildUsageDashboardViewModel,
  getEffectiveGranularity,
  resolveStoredTimeRange,
  SELECTED_TIME_RANGE_STORAGE_KEY,
  TIME_RANGES,
} from "@/features/usage-intelligence/view-model"
import type { TimeRange, TimeGranularity } from "@/types/api"
import { Clock, CalendarRange, Filter, Pin } from "lucide-react"
import { cn } from "@/lib/utils"

export const Route = createLazyFileRoute("/")({
  component: DashboardPage,
})

function DashboardPage() {
  const [range, setRange] = useState<TimeRange>(() => readStoredTimeRange())
  const [granularity, setGranularity] = useState<TimeGranularity | null>(null)
  const [provider, setProvider] = useState("")
  const [trendView, setTrendView] = useState<"cost-token" | "requests-token" | "tokens">("cost-token")
  const [leaderboardScope, setLeaderboardScope] = useState<"account" | "api-key">("api-key")

  const g = getEffectiveGranularity(range, granularity)
  const { data, isLoading, error } = useAnalytics(range, g, provider)
  const { data: healthOverviewData, isLoading: isHealthLoading } = useUsageOverview("24h", provider)

  useEffect(() => {
    writeStoredTimeRange(range)
  }, [range])

  const summary = data?.summary
  const comparison = data?.comparison
  const {
    trend,
    leaderboardRows,
    providerOptions,
    fixedHeatmap,
    serviceHealth,
    leaderboardSortLabel,
    kpiData,
  } = useMemo(
    () => buildUsageDashboardViewModel({
      analytics: data,
      healthOverview: healthOverviewData,
      leaderboardScope,
    }),
    [data, healthOverviewData, leaderboardScope],
  )

  return (
    <div className="animate-slide-up mx-auto max-w-7xl space-y-6">
      {/* Header */}
      <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Usage Intelligence
          </p>
          <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight text-foreground">
            Dashboard
          </h1>
        </div>
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
          {/* Time Range */}
          <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
            {TIME_RANGES.map((tr) => (
              <button
                key={tr.value}
                onClick={() => {
                  setRange(tr.value)
                  setGranularity(null)
                }}
                className={cn(
                  "shrink-0 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  range === tr.value
                    ? "bg-terracotta-500 text-white"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                )}
              >
                {tr.label}
              </button>
            ))}
          </div>

          {/* Granularity Toggle */}
          <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
            <button
              onClick={() => setGranularity("hour")}
              className={cn(
                "flex shrink-0 items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                g === "hour"
                  ? "bg-terracotta-500 text-white"
                  : "text-muted-foreground hover:bg-muted"
              )}
            >
              <Clock className="h-3 w-3" />
              Hour
            </button>
            <button
              onClick={() => setGranularity("day")}
              className={cn(
                "flex shrink-0 items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                g === "day"
                  ? "bg-terracotta-500 text-white"
                  : "text-muted-foreground hover:bg-muted"
              )}
            >
              <CalendarRange className="h-3 w-3" />
              Day
            </button>
          </div>

          {/* Scope indicator */}
          <span className="hidden items-center gap-1 text-[10px] text-muted-foreground/60 sm:inline-flex">
            <Clock className="h-3 w-3" />
            Applies to KPIs, Trend & Leaderboard
          </span>
        </div>
      </header>

      {/* Provider Filter */}
      {providerOptions.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5">
          <Filter className="h-3.5 w-3.5 text-muted-foreground" />
          <button
            onClick={() => setProvider("")}
            className={cn(
              "rounded-full px-3 py-1 text-xs font-medium transition-colors",
              provider === ""
                ? "bg-foreground text-background"
                : "bg-muted text-muted-foreground hover:bg-muted/80"
            )}
          >
            All
          </button>
          {providerOptions.map((opt) => (
            <button
              key={opt.provider}
              onClick={() => setProvider(opt.provider)}
              className={cn(
                "rounded-full px-3 py-1 text-xs font-medium transition-colors",
                provider === opt.provider
                  ? "bg-foreground text-background"
                  : "bg-muted text-muted-foreground hover:bg-muted/80"
              )}
            >
              {opt.provider}
              <span className="ml-1 text-[10px] opacity-60">
                {formatCompact(opt.request_count, 0)}
              </span>
            </button>
          ))}
        </div>
      )}

      {/* KPI Cards — 5 compact cards */}
      <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <KpiCard
          label="Cost"
          rawValue={summary?.total_cost}
          formatter={formatCost}
          caption={summary?.cost_status}
          comparison={comparison?.has_previous_period ? formatComparison(comparison.total_cost_change_pct, "%") : undefined}
          sparkline={kpiData?.cost}
          isLoading={isLoading}
          tone="terracotta"
        />
        <KpiCard
          label="Tokens"
          rawValue={summary?.total_tokens}
          formatter={(n) => formatCompact(n, 2)}
          comparison={comparison?.has_previous_period ? formatComparison(comparison.total_tokens_change_pct, "%") : undefined}
          sparkline={kpiData?.tokens}
          isLoading={isLoading}
          tone="blue"
        />
        <KpiCard
          label="Requests"
          rawValue={summary?.request_count}
          formatter={(n) => n.toLocaleString("en")}
          comparison={comparison?.has_previous_period ? formatComparison(comparison.request_count_change_pct, "%") : undefined}
          sparkline={kpiData?.requests}
          isLoading={isLoading}
          tone="violet"
        />
        <KpiCard
          label="Success"
          rawValue={summary?.success_rate}
          formatter={(n) => `${n.toFixed(1)}%`}
          caption={`${summary?.failure_count ?? 0} failed`}
          comparison={comparison?.has_previous_period ? formatComparison(comparison.success_rate_change_pp, "pp") : undefined}
          sparkline={kpiData?.successRate}
          isLoading={isLoading}
          tone="green"
        />
        <KpiCard
          label="Cache"
          rawValue={summary?.cache_read_share}
          formatter={(n) => `${n.toFixed(1)}%`}
          caption={summary?.cache_read_share_state === "available" ? "Cache Read Share" : summary?.cache_read_share_state?.replace(/_/g, " ")}
          isLoading={isLoading}
          tone="amber"
        />
      </div>

      {/* Trend + Leaderboard — side by side */}
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
                  onClick={() => setTrendView(item.value as typeof trendView)}
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
            {isLoading ? (
              <Skeleton className="h-[260px] w-full" />
            ) : error ? (
              <div className="flex h-[260px] items-center justify-center text-sm text-red-500">
                Failed to load trend data
              </div>
            ) : (
              <div className="h-[260px]">
                <TrendChart data={trend} range={range} mode={trendView} />
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
                    onClick={() => setLeaderboardScope(item.value as typeof leaderboardScope)}
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
            {isLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            ) : (
              <KeyLeaderboard data={leaderboardRows} />
            )}
          </CardContent>
        </Card>
      </div>

      {/* Divider — Fixed overview */}
      <div className="flex items-center gap-3">
        <div className="h-px flex-1 bg-border" />
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <Pin className="h-3 w-3" />
          Fixed overview
        </span>
        <div className="h-px flex-1 bg-border" />
      </div>

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
          {isLoading ? (
            <Skeleton className="h-[260px] w-full" />
          ) : fixedHeatmap ? (
            <Heatmap data={fixedHeatmap} />
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
            {isHealthLoading ? (
              <Skeleton className="h-[180px] w-full" />
            ) : serviceHealth ? (
              <HealthGrid data={serviceHealth} />
            ) : (
              <div className="flex h-[180px] items-center justify-center text-sm text-muted-foreground">
                No health data
              </div>
            )}
          </CardContent>
        </Card>

        <RequestEvidence provider={provider} />
      </div>
    </div>
  )
}

function readStoredTimeRange(): TimeRange {
  if (typeof window === "undefined") {
    return resolveStoredTimeRange(null)
  }
  try {
    return resolveStoredTimeRange(window.localStorage.getItem(SELECTED_TIME_RANGE_STORAGE_KEY))
  } catch {
    return resolveStoredTimeRange(null)
  }
}

function writeStoredTimeRange(range: TimeRange) {
  try {
    window.localStorage.setItem(SELECTED_TIME_RANGE_STORAGE_KEY, range)
  } catch {
    // Browser storage can be unavailable in private or restricted contexts.
  }
}

/* ─── KPI Card ─── */

interface KpiCardProps {
  label: string
  rawValue?: number
  formatter?: (n: number) => string
  caption?: string
  comparison?: string
  sparkline?: (number | null)[]
  isLoading: boolean
  tone: "terracotta" | "blue" | "violet" | "green" | "amber"
}

const toneStyles = {
  terracotta: "text-terracotta-700 bg-terracotta-50 border-terracotta-200",
  blue: "text-blue-700 bg-blue-50 border-blue-200",
  violet: "text-violet-700 bg-violet-50 border-violet-200",
  green: "text-emerald-700 bg-emerald-50 border-emerald-200",
  amber: "text-amber-700 bg-amber-50 border-amber-200",
}

function KpiCard({ label, rawValue, formatter, caption, comparison, sparkline, isLoading, tone }: KpiCardProps) {
  const animated = useCountUp(rawValue ?? 0, {
    duration: 900,
    decimals: formatter === formatCost ? 2 : 0,
    enabled: rawValue !== undefined,
  })
  const display = rawValue !== undefined && formatter ? formatter(animated) : "—"

  return (
    <Card>
      <CardContent className="p-5">
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-8 w-28" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : (
          <>
            <div
              className={`mb-3 inline-flex rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider ${toneStyles[tone]}`}
            >
              {label}
            </div>
            <p className="font-serif text-2xl font-semibold tracking-tight">{display}</p>
            {comparison && (
              <p className="mt-1 text-xs font-medium text-muted-foreground">{comparison}</p>
            )}
            <div className="mt-3 h-10">
              {sparkline && <Sparkline data={sparkline} />}
            </div>
            {caption && (
              <p className="mt-2 text-xs text-muted-foreground">{caption}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
