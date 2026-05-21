import { AlertTriangle, Gauge, RefreshCw } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { buildLiveCapacityRows, type LiveCapacityMetric, type LiveCapacityStatus } from "@/features/usage-intelligence/live-capacity"
import { useLiveCapacity } from "@/hooks/useQuota"
import { cn } from "@/lib/utils"

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

  return (
    <div
      className={cn(
        "group flex min-h-[176px] min-w-0 flex-col rounded-lg border border-border bg-background/70 p-3 text-sm transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-0.5 hover:border-terracotta-500/25 hover:shadow-sm",
        row.status === "failed" && "border-red-500/25 bg-red-500/[0.025]",
        row.status !== "failed" && row.isConstrained && "border-amber-500/30 bg-amber-500/[0.03]",
        isRowRefreshing && "border-amber-500/25 shadow-[0_0_0_1px_rgba(245,158,11,0.08)]",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <Badge variant="outline" className="shrink-0 text-[10px]">
            {row.provider || row.type || "unknown"}
          </Badge>
          {row.planType ? <Badge variant="terracotta" className="text-[10px]">{row.planType}</Badge> : null}
          <StatusBadge status={row.status} label={row.statusLabel} />
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {hasAttention ? (
            <AlertTriangle
              className={cn("h-4 w-4", row.status === "failed" ? "text-red-600" : "text-amber-600")}
              aria-label="Capacity attention required"
            />
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7 opacity-70 transition-opacity group-hover:opacity-100"
            onClick={onRefresh}
            disabled={isRowRefreshing}
            aria-label={`Refresh ${row.alias || row.displayName || row.name || row.authIndex}`}
            title="Refresh this account"
          >
            <RefreshCw className={cn("h-3.5 w-3.5", isRowRefreshing && "animate-spin")} />
          </Button>
        </div>
      </div>

      <div className="mt-3 min-w-0">
        <p className="mt-2 truncate font-medium">{row.alias || row.displayName || row.name || row.authIndex}</p>
        <p className="mt-0.5 truncate text-xs text-muted-foreground">{row.authIndex}</p>
      </div>

      <div className="mt-4 grid gap-2">
        <MetricMeter title={primaryMetric?.label ?? "5h"} metric={primaryMetric} />
        <MetricMeter title={secondaryMetric?.label ?? "Weekly"} metric={secondaryMetric} />
      </div>

      <div className="mt-auto flex items-center justify-between gap-3 border-t border-border/70 pt-3 text-xs">
        <span className="text-muted-foreground">Reset</span>
        <span className="min-w-0 truncate font-medium">{row.resetLabel}</span>
      </div>
    </div>
  )
}

function StatusBadge({ status, label }: { status: LiveCapacityStatus; label: string }) {
  const variant =
    status === "failed"
      ? "red"
      : status === "refreshing"
        ? "amber"
        : status === "cached"
          ? "green"
          : status === "unsupported"
            ? "secondary"
            : "outline"
  return (
    <Badge variant={variant} className="text-[10px]">
      {label}
    </Badge>
  )
}

function MetricMeter({ title, metric }: { title: string; metric?: LiveCapacityMetric }) {
  const progress = metric?.progress ?? null

  return (
    <div className="min-w-0 rounded-md border border-border/70 bg-muted/20 p-2">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="text-muted-foreground">{title}</span>
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
