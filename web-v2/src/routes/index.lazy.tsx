import { createLazyFileRoute } from "@tanstack/react-router"
import { useState, useMemo } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAnalytics } from "@/hooks/useAnalytics"
import { useCountUp } from "@/hooks/useCountUp"
import { Sparkline } from "@/components/charts/sparkline"
import { TrendChart } from "@/components/charts/trend-chart"
import { KeyLeaderboard } from "@/components/charts/key-leaderboard"
import { Heatmap } from "@/components/charts/heatmap"
import { HealthTimeline } from "@/components/charts/health-timeline"
import { formatCost, formatCompact, formatComparison } from "@/lib/format"
import type { TimeRange, TimeGranularity } from "@/types/api"
import { Clock, CalendarRange, Filter } from "lucide-react"
import { cn } from "@/lib/utils"

export const Route = createLazyFileRoute("/")({
  component: DashboardPage,
})

const TIME_RANGES: { value: TimeRange; label: string }[] = [
  { value: "today", label: "Today" },
  { value: "yesterday", label: "Yesterday" },
  { value: "24h", label: "Last 24h" },
  { value: "7d", label: "7 days" },
  { value: "30d", label: "30 days" },
]

function defaultGranularity(range: TimeRange): TimeGranularity {
  if (range === "30d") return "day"
  return "hour"
}

function DashboardPage() {
  const [range, setRange] = useState<TimeRange>("7d")
  const [granularity, setGranularity] = useState<TimeGranularity | null>(null)
  const [provider, setProvider] = useState("")

  const g = granularity ?? defaultGranularity(range)
  const { data, isLoading, error } = useAnalytics(range, g, provider)

  const summary = data?.summary
  const comparison = data?.comparison
  const trend = data?.trend ?? []
  const keyAliases = data?.key_alias_breakdown ?? []
  const heatmap = data?.heatmap
  const providerOptions = data?.provider_options ?? []

  const primaryMeasure: "cost" | "tokens" =
    summary?.cost_status === "available" ? "cost" : "tokens"

  const kpiData = useMemo(() => {
    if (!trend.length) return null
    return {
      cost: trend.map((p) => (p.cost_status === "unavailable" ? null : p.total_cost)),
      tokens: trend.map((p) => p.total_tokens),
      requests: trend.map((p) => p.request_count),
      successRate: trend.map((p) => {
        const success = Math.max(p.request_count - p.failure_count, 0)
        return p.request_count > 0 ? (success / p.request_count) * 100 : 0
      }),
    }
  }, [trend])

  const healthData = useMemo(() => {
    return trend.map((p) => {
      const success = Math.max(p.request_count - p.failure_count, 0)
      return {
        label: p.label,
        success,
        failure: p.failure_count,
        rate: p.request_count > 0 ? Number(((success / p.request_count) * 100).toFixed(1)) : 0,
      }
    })
  }, [trend])

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
        <div className="flex flex-wrap items-center gap-2">
          {/* Time Range */}
          <div className="flex items-center rounded-lg border border-border bg-card p-1">
            {TIME_RANGES.map((tr) => (
              <button
                key={tr.value}
                onClick={() => {
                  setRange(tr.value)
                  setGranularity(null)
                }}
                className={cn(
                  "rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
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
          <div className="flex items-center rounded-lg border border-border bg-card p-1">
            <button
              onClick={() => setGranularity("hour")}
              className={cn(
                "flex items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
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
                "flex items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                g === "day"
                  ? "bg-terracotta-500 text-white"
                  : "text-muted-foreground hover:bg-muted"
              )}
            >
              <CalendarRange className="h-3 w-3" />
              Day
            </button>
          </div>
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
          caption={summary?.cache_read_share_state === "available" ? "Cache hit rate" : summary?.cache_read_share_state?.replace(/_/g, " ")}
          isLoading={isLoading}
          tone="amber"
        />
      </div>

      {/* Trend Chart */}
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <CardTitle>Cost & Token Trend</CardTitle>
            <CardDescription>Cost as filled area, tokens as dotted overlay</CardDescription>
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
              <TrendChart data={trend} range={range} />
            </div>
          )}
        </CardContent>
      </Card>

      {/* Health + Leaderboard — side by side */}
      <div className="grid gap-6 lg:grid-cols-[1fr_400px]">
        {/* Request Health */}
        <Card>
          <CardHeader className="flex flex-row items-start justify-between gap-4 pb-2">
            <div>
              <CardTitle>Request Health</CardTitle>
              <CardDescription>Success/failure per bucket</CardDescription>
            </div>
            {summary && summary.failure_count > 0 ? (
              <Badge variant="amber">{summary.failure_count} failed</Badge>
            ) : (
              <Badge variant="green">No failures</Badge>
            )}
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-[140px] w-full" />
            ) : (
              <div className="h-[140px]">
                <HealthTimeline data={trend} granularity={g} />
              </div>
            )}
          </CardContent>
        </Card>

        {/* Key Leaderboard */}
        <Card>
          <CardHeader className="flex flex-row items-start justify-between pb-2">
            <div>
              <CardTitle>Key Leaderboard</CardTitle>
              <CardDescription>Top contributors by alias</CardDescription>
            </div>
            <Badge variant={primaryMeasure === "cost" ? "terracotta" : "blue"}>
              {primaryMeasure === "cost" ? "Cost" : "Tokens"}
            </Badge>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            ) : (
              <KeyLeaderboard data={keyAliases} measure={primaryMeasure} />
            )}
          </CardContent>
        </Card>
      </div>

      {/* Heatmap */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle>Activity Heatmap</CardTitle>
          <CardDescription>Hourly usage density across days</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-[260px] w-full" />
          ) : heatmap ? (
            <Heatmap data={heatmap} />
          ) : (
            <div className="flex h-[260px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
              No heatmap data
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
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
