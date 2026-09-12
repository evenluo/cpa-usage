import { Link } from "@tanstack/react-router"
import { ArrowRight, ArrowUpRight, ChevronDown } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { formatCompact, formatCost, formatDate, formatPercent } from "@/lib/format"
import type { UsageModelMappingSummary } from "@/features/usage-intelligence/model-mapping-summary"
import type { CostStatus, UsageModelMapping, UsageModelMappingDistribution } from "@/types/api"

interface ModelMappingsProps {
  summary?: UsageModelMappingSummary
  data?: UsageModelMappingDistribution
  isSummaryLoading: boolean
  summaryError: unknown
  isDetailsLoading: boolean
  detailsError: unknown
  onExpandedChange: (expanded: boolean) => void
  onRetrySummary: () => void
  onRetryDetails: () => void
}

export function ModelMappings({
  summary,
  data,
  isSummaryLoading,
  summaryError,
  isDetailsLoading,
  detailsError,
  onExpandedChange,
  onRetrySummary,
  onRetryDetails,
}: ModelMappingsProps) {
  if (summary && summary.total_attempts > 0) {
    return (
      <Card className="min-w-0 overflow-hidden">
        <details className="group" onToggle={(event) => onExpandedChange(event.currentTarget.open)}>
          <summary className="grid cursor-pointer list-none grid-cols-[minmax(0,1fr)_auto] items-start gap-x-4 gap-y-2 rounded-xl px-4 py-4 transition-colors hover:bg-muted/40 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring sm:px-6 [&::-webkit-details-marker]:hidden">
            <CardTitle>Model mappings</CardTitle>
            <ChevronDown className="row-span-2 h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" aria-hidden="true" />
            <span className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
              <span title={`${formatDate(summary.window_start)} – ${formatDate(summary.window_end)}`}>Last 24h</span>
              <span><strong className="font-medium text-foreground">{formatCompact(summary.displayed_mappings)}</strong> mappings</span>
              <span><strong className="font-medium text-foreground">{formatPercent(summary.alias_coverage)}</strong> have an alias</span>
              <span><strong className="font-medium text-foreground">{formatCompact(summary.missing_alias_attempts)}</strong> without alias</span>
            </span>
          </summary>

          <CardContent className="border-t border-border px-4 pb-5 pt-5 sm:px-6">
            {data ? <div className="space-y-5">
              <div className="flex flex-wrap items-start justify-between gap-2 text-xs text-muted-foreground">
                <p>Share of attempts with an alias</p>
                <details className="max-w-lg">
                  <summary className="cursor-pointer rounded-sm underline decoration-dotted underline-offset-4 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">About these mappings</summary>
                  <p className="mt-2 leading-relaxed">Same name does not mean it was routed that way.</p>
                </details>
              </div>

              <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span>{formatCompact(data.observed_alias_attempts)} of {formatCompact(data.total_attempts)} attempts with an alias</span>
                <span>Cost · {data.observed_alias_attempts > 0 ? formatObservedCost(data.observed_total_cost, data.observed_cost_status) : "No aliases"}</span>
              </div>

              {data.mappings.length === 0 ? (
                <div className="rounded-lg border border-dashed border-border px-3 py-8 text-center text-sm text-muted-foreground">No alias mappings</div>
              ) : (
                <div className="space-y-2">
                  {data.mappings.map((row) => (
                    <MappingRow
                      key={`${row.model_alias}:${row.model}:${row.provider}`}
                      row={row}
                      observedAttempts={data.observed_alias_attempts}
                      windowEnd={data.window_end}
                    />
                  ))}
                </div>
              )}

              {data.other_attempts > 0 ? (
                <p className="text-xs text-muted-foreground">Other observed attempts · {formatCompact(data.other_attempts)}</p>
              ) : null}
            </div> : isDetailsLoading ? (
              <div className="space-y-2" aria-label="Loading model mapping details">
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </div>
            ) : detailsError ? (
              <div className="flex min-h-24 flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
                <span>Couldn't load model mapping details</span>
                <Button type="button" size="sm" variant="outline" onClick={onRetryDetails}>Retry</Button>
              </div>
            ) : (
              <Skeleton className="h-16 w-full" />
            )}
          </CardContent>
        </details>
        {summaryError ? <RefreshFailure onRetry={onRetrySummary} /> : null}
        {data && detailsError ? <RefreshFailure onRetry={onRetryDetails} /> : null}
      </Card>
    )
  }

  return (
    <Card className="min-w-0 overflow-hidden">
      <CardHeader className="pb-4">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <CardTitle>Model mappings</CardTitle>
          <span className="text-xs text-muted-foreground">Last 24h</span>
        </div>
      </CardHeader>
      <CardContent>
        {!summary && isSummaryLoading ? (
          <Skeleton className="h-12 w-full" />
        ) : !summary && summaryError ? (
          <div className="flex min-h-24 flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
            <span>Couldn't load model mappings</span>
            <Button type="button" size="sm" variant="outline" onClick={onRetrySummary}>Retry</Button>
          </div>
        ) : (
          <div className="flex min-h-16 items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">No attempts in 24h</div>
        )}
      </CardContent>
      {summary && summaryError ? <RefreshFailure onRetry={onRetrySummary} /> : null}
    </Card>
  )
}

function MappingRow({ row, observedAttempts, windowEnd }: { row: UsageModelMapping; observedAttempts: number; windowEnd: string }) {
  const attemptShare = observedAttempts > 0 ? row.attempt_count / observedAttempts * 100 : 0
  const content = (
    <>
      <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-sm">
        <span className="[overflow-wrap:anywhere] font-semibold">{row.model_alias}</span>
        <ArrowRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span className="[overflow-wrap:anywhere] font-medium">{row.model || "Actual model unavailable"}</span>
        <span className="inline-flex min-w-0 gap-1 text-xs text-muted-foreground">
          <span aria-hidden="true">/</span>
          <span className="[overflow-wrap:anywhere]">{row.provider || "Provider unavailable"}</span>
        </span>
        {row.model && row.model_alias === row.model ? (
          <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">Same name</span>
        ) : null}
      </div>

      <div className="mt-3 flex items-center gap-3">
        <div
          className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-muted"
          role="img"
          aria-label={`${formatPercent(attemptShare)} of attempts with an observed alias`}
        >
          <div className="h-full rounded-full bg-terracotta-500" style={{ width: `${Math.min(attemptShare, 100)}%` }} />
        </div>
        <span className="w-24 shrink-0 text-right text-xs font-medium tabular-nums">
          {formatCompact(row.attempt_count)} · {formatPercent(attemptShare)}
        </span>
      </div>

      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
        <span>Failures · {formatPercent(row.failure_share)}</span>
        <span title={row.latency_sample_count > 0 ? `${row.latency_sample_count.toLocaleString("en")} ${row.latency_sample_count === 1 ? "sample" : "samples"}` : "No latency samples"}>
          Mean latency · {row.latency_sample_count > 0 ? `${row.mean_latency_ms.toLocaleString("en", { maximumFractionDigits: 1 })} ms` : "—"}
        </span>
        <span>Cost · {formatObservedCost(row.total_cost, row.cost_status)}</span>
      </div>
    </>
  )
  const className = "relative block min-w-0 rounded-lg border border-border p-3 pr-9 transition-colors"

  if (!row.model || !row.provider) return <div className={className}>{content}</div>
  return (
    <Link
      to="/requests"
      search={{ provider: row.provider, model: row.model, modelAlias: row.model_alias, account: "", endpoint: "", status: "", requestId: "", windowEnd, result: "" }}
      aria-label={`Inspect ${row.model_alias} to ${row.model} attempts`}
      className={`${className} hover:bg-muted/40 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-terracotta-500`}
    >
      {content}<ArrowUpRight className="absolute right-3 top-3 h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
    </Link>
  )
}

function RefreshFailure({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border px-4 py-2.5 text-xs text-red-500 sm:px-6" role="alert">
      <span>Refresh failed. Showing last result.</span>
      <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry</Button>
    </div>
  )
}

function formatObservedCost(value: number, status: CostStatus) {
  if (status === "unavailable") return "Unavailable"
  return `${formatCost(value)}${status === "partial" ? " · Partial" : ""}`
}
