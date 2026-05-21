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
          <div className="max-h-[340px] space-y-2 overflow-y-auto pr-1">
            {rows.map((row) => (
              <LiveCapacityRowItem
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

function LiveCapacityRowItem({
  row,
  onRefresh,
}: {
  row: ReturnType<typeof buildLiveCapacityRows>[number]
  onRefresh: () => void
}) {
  const primaryMetric = row.fiveHour ?? row.additionalMetrics[0]
  const secondaryMetric = row.weekly ?? row.additionalMetrics[1]
  const isRowRefreshing = row.status === "refreshing"

  return (
    <div
      className="grid min-w-0 gap-3 rounded-lg border border-border bg-background/60 p-3 text-sm lg:grid-cols-[minmax(190px,0.9fr)_minmax(130px,0.7fr)_minmax(130px,0.7fr)_minmax(90px,0.45fr)_auto]"
    >
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <Badge variant="outline" className="shrink-0 text-[10px]">
            {row.provider || row.type || "unknown"}
          </Badge>
          {row.planType ? <Badge variant="terracotta" className="text-[10px]">{row.planType}</Badge> : null}
          <StatusBadge status={row.status} label={row.statusLabel} />
        </div>
        <p className="mt-2 truncate font-medium">{row.alias || row.displayName || row.name || row.authIndex}</p>
        <p className="mt-0.5 truncate text-xs text-muted-foreground">{row.authIndex}</p>
      </div>

      <MetricCell title={primaryMetric?.label ?? "5h"} metric={primaryMetric} />
      <MetricCell title={secondaryMetric?.label ?? "Weekly"} metric={secondaryMetric} />

      <div className="min-w-0 lg:text-right">
        <p className="text-xs text-muted-foreground">Reset</p>
        <p className="mt-1 truncate font-medium">{row.resetLabel}</p>
      </div>

      <div className="flex items-start justify-end gap-1">
        {row.isConstrained || row.status === "failed" ? (
          <AlertTriangle className="mt-2 h-4 w-4 text-amber-600" aria-label="Capacity attention required" />
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={onRefresh}
          disabled={isRowRefreshing}
          aria-label={`Refresh ${row.alias || row.displayName || row.name || row.authIndex}`}
          title="Refresh this account"
        >
          <RefreshCw className={cn("h-3.5 w-3.5", isRowRefreshing && "animate-spin")} />
        </Button>
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

function MetricCell({ title, metric }: { title: string; metric?: LiveCapacityMetric }) {
  return (
    <div className="min-w-0">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="text-muted-foreground">{title}</span>
        <span className="truncate font-medium">{metric?.valueLabel ?? "-"}</span>
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted">
        {metric?.progress !== null && metric?.progress !== undefined ? (
          <div
            className={cn(
              "h-full rounded-full",
              metric.tone === "red" && "bg-red-500",
              metric.tone === "amber" && "bg-amber-500",
              metric.tone === "green" && "bg-emerald-500",
              metric.tone === "muted" && "bg-muted-foreground/40",
            )}
            style={{ width: `${metric.progress}%` }}
          />
        ) : null}
      </div>
    </div>
  )
}
