import { AlertTriangle, CalendarDays, Clock, Gauge, RefreshCw, Timer } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { buildLiveCapacityRows, type LiveCapacityMetric } from "@/features/usage-intelligence/live-capacity"
import { useLiveCapacity } from "@/hooks/useQuota"
import { cn } from "@/lib/utils"
import { ProviderBrandIcon } from "./provider-brand-icon"

export function LiveCapacityCard({ provider }: { provider: string }) {
  const { identities, cachedQuota, taskStates, refresh, refreshLimit, isLoading, isRefreshing, error } = useLiveCapacity(provider)
  const rows = buildLiveCapacityRows({ identities, cachedQuota, taskStates })
  const refreshLabel = identities.length > refreshLimit ? `Refresh first ${refreshLimit}` : "Refresh"

  return (
    <Card>
      <CardHeader className="flex flex-col items-start justify-between gap-3 pb-3 sm:flex-row sm:items-center">
        <div>
          <CardTitle className="flex items-center gap-2">
            Live Capacity
            <Gauge className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Fixed live capacity probe" />
          </CardTitle>
          <CardDescription>Cached auth-file capacity probe</CardDescription>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="blue">live probe</Badge>
          <Badge variant="outline">fixed</Badge>
          {identities.length > refreshLimit ? <Badge variant="amber">max {refreshLimit}</Badge> : null}
          <Button type="button" variant="outline" size="sm" onClick={() => refresh()} disabled={identities.length === 0}>
            <RefreshCw className={cn("mr-1.5 h-3.5 w-3.5", isRefreshing && "animate-spin")} />
            {refreshLabel}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : error ? (
          <div className="flex h-[140px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
            Failed to load live capacity
          </div>
        ) : identities.length === 0 ? (
          <div className="flex h-[140px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No auth-file accounts
          </div>
        ) : (
          <div className="grid max-h-[560px] grid-cols-1 gap-3 overflow-y-auto pr-1 md:grid-cols-2 xl:grid-cols-3">
            {rows.map((row) => (
              <LiveCapacityAccountTile
                key={row.authIndex}
                row={row}
                onRefresh={() => refresh(row.authIndex)}
              />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function LiveCapacityAccountTile({
  row,
  onRefresh,
}: {
  row: ReturnType<typeof buildLiveCapacityRows>[number]
  onRefresh: () => void
}) {
  const primaryMetric = row.fiveHour ?? row.additionalMetrics[0]
  const secondaryMetric = row.weekly ?? row.additionalMetrics[1]
  const isRowRefreshing = row.status === "refreshing"
  const hasAttention = row.isConstrained || row.status === "failed"
  const accountTitle = row.alias || row.displayName || row.name || row.authIndex
  const planLabel = row.planType || "no plan"
  const attentionLabel = row.status === "failed"
    ? `Refresh failed: ${row.error || row.statusLabel}`
    : row.isConstrained
      ? "Capacity constrained"
      : undefined

  return (
    <div
      className={cn(
        "group flex min-h-[190px] min-w-0 flex-col rounded-lg border border-border bg-background/70 p-3 text-sm transition-[background-color,border-color,box-shadow] duration-300 hover:border-terracotta-500/25 hover:shadow-sm",
        row.status === "failed" && "border-red-500/25 bg-red-500/[0.025]",
        row.status !== "failed" && row.isConstrained && "border-amber-500/30 bg-amber-500/[0.03]",
        row.status !== "failed" && !row.isConstrained && row.isPriorityAccount && "border-terracotta-500/30 shadow-[inset_2px_0_0_rgba(192,80,62,0.45)]",
        isRowRefreshing && "border-amber-500/25 shadow-[0_0_0_1px_rgba(245,158,11,0.08)]",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-start gap-2">
          <div
            className="flex h-7 max-w-[128px] shrink-0 items-center gap-1.5 rounded-md border border-border/70 bg-muted/20 px-2"
            title={row.providerLabel}
          >
            <ProviderBrandIcon providerKind={row.providerKind} label={row.providerLabel} />
            <span className="truncate text-[11px] font-medium leading-none text-foreground">{row.providerLabel}</span>
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-1.5">
              <p className="truncate font-medium leading-5" title={accountTitle}>{accountTitle}</p>
              {row.priorityLabel ? (
                <Badge variant="terracotta" className="shrink-0 px-1.5 py-0 text-[10px] leading-4">
                  {row.priorityLabel}
                </Badge>
              ) : null}
            </div>
            <p className="mt-0.5 truncate text-xs text-muted-foreground" title={row.authIndex}>{row.authIndex}</p>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {hasAttention ? (
            <span title={attentionLabel}>
              <AlertTriangle
                className={cn("h-4 w-4", row.status === "failed" ? "text-red-600" : "text-amber-600")}
                aria-label={attentionLabel}
              />
            </span>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7 opacity-70 transition-opacity group-hover:opacity-100"
            onClick={onRefresh}
            disabled={isRowRefreshing}
            aria-label={`Refresh ${accountTitle}`}
            title="Refresh this account"
          >
            <RefreshCw className={cn("h-3.5 w-3.5", isRowRefreshing && "animate-spin")} />
          </Button>
        </div>
      </div>

      <p className="mt-2 truncate text-[11px] text-muted-foreground/80" title={`${row.providerLabel} · ${planLabel} · ${row.statusLabel}`}>
        {row.providerLabel} · {planLabel} · {row.statusLabel}
      </p>

      <div className="mt-3 grid gap-2">
        <MetricMeter title={primaryMetric?.label ?? "5h"} metric={primaryMetric} iconKind={primaryMetric === row.fiveHour ? "5h" : undefined} />
        <MetricMeter title={secondaryMetric?.label ?? "Weekly"} metric={secondaryMetric} iconKind={secondaryMetric === row.weekly ? "weekly" : undefined} />
      </div>
    </div>
  )
}

function MetricMeter({
  title,
  metric,
  iconKind,
}: {
  title: string
  metric?: LiveCapacityMetric
  iconKind?: "5h" | "weekly"
}) {
  const progress = metric?.progress ?? null
  const resetLabel = metric?.resetLabel ?? "-"
  const resetText = resetLabel === "-" ? "-" : `reset ${resetLabel}`
  const WindowIcon = iconKind === "5h" ? Timer : iconKind === "weekly" ? CalendarDays : null

  return (
    <div className="min-w-0 rounded-md border border-border/70 bg-muted/20 p-2">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
          {WindowIcon ? <WindowIcon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" /> : null}
          <span className="truncate">{title}</span>
        </span>
        <span className="truncate font-medium">{metric?.valueLabel ?? "-"}</span>
      </div>
      <div
        className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"
        aria-label={metric ? `${title}: ${metric.valueLabel}` : `${title}: no capacity reading`}
      >
        {progress !== null ? (
          <div
            className={cn("h-full rounded-full transition-[width,background-color] duration-500", metricToneClass(metric?.tone))}
            style={{ width: `${progress}%` }}
          />
        ) : null}
      </div>
      <div className="mt-1.5 flex min-w-0 items-center gap-1 text-[11px] text-muted-foreground" title={resetText}>
        <Clock className="h-3 w-3 shrink-0" aria-hidden="true" />
        <span className="truncate">{resetText}</span>
      </div>
    </div>
  )
}

function metricToneClass(tone: LiveCapacityMetric["tone"] | undefined): string {
  switch (tone) {
    case "red":
      return "bg-red-500"
    case "amber":
      return "bg-amber-500"
    case "green":
      return "bg-emerald-500"
    case "muted":
      return "bg-muted-foreground/40"
    default:
      return "bg-muted-foreground/30"
  }
}
