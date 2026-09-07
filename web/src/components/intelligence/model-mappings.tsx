import { Link } from "@tanstack/react-router"
import { ArrowUpRight, Pin } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { formatCompact, formatCost, formatPercent } from "@/lib/format"
import type { CostStatus, UsageModelMapping, UsageModelMappingDistribution } from "@/types/api"

interface ModelMappingsProps {
  data?: UsageModelMappingDistribution
  isLoading: boolean
  error: unknown
  onRetry: () => void
}

export function ModelMappings({ data, isLoading, error, onRetry }: ModelMappingsProps) {
  const hasCompleteData = data !== undefined
  return (
    <Card className="min-w-0 overflow-hidden">
      <CardHeader className="flex flex-col items-start justify-between gap-3 sm:flex-row">
        <div>
          <CardTitle className="flex items-center gap-2">
            Observed model mappings
            <Pin className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Fixed 24-hour view" />
          </CardTitle>
          <CardDescription>CPA alias labels observed beside the actual model and provider for each upstream attempt.</CardDescription>
        </div>
        <div className="flex items-center gap-2">
          {hasCompleteData && error ? <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry refresh</Button> : null}
          <Badge variant="terracotta">24h fixed</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {!hasCompleteData && isLoading ? (
          <Skeleton className="h-44 w-full" />
        ) : !hasCompleteData && error ? (
          <div className="flex h-40 flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
            <span>Failed to load observed model mappings</span>
            <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry model mappings</Button>
          </div>
        ) : !data || data.total_attempts === 0 ? (
          <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">No attempts in the last 24 hours</div>
        ) : (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted-foreground">
              <span><strong className="text-foreground">{formatCompact(data.observed_alias_attempts)}</strong> of {formatCompact(data.total_attempts)} attempts have an observed alias ({formatPercent(data.alias_coverage)})</span>
              <span>{formatCompact(data.missing_alias_attempts)} missing alias</span>
              <span>Observed Cost: {data.observed_alias_attempts > 0 ? formatObservedCost(data.observed_total_cost, data.observed_cost_status) : "No observed alias population"}</span>
            </div>
            {data.mappings.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border px-3 py-8 text-center text-sm text-muted-foreground">No observed alias mappings in scope</div>
            ) : (
              <div className="grid gap-2 lg:grid-cols-2">
                {data.mappings.map((row) => <MappingRow key={`${row.model_alias}:${row.model}:${row.provider}`} row={row} windowEnd={data.window_end} />)}
              </div>
            )}
            {data.other_attempts > 0 ? <p className="text-xs text-muted-foreground">{formatCompact(data.other_attempts)} lower-ranked observed attempts are outside this bounded breakdown.</p> : null}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function MappingRow({ row, windowEnd }: { row: UsageModelMapping; windowEnd: string }) {
  const content = (
    <>
      <div className="min-w-0 sm:flex-1">
        <p className="break-words text-sm font-semibold">{row.model_alias}</p>
        <p className="mt-0.5 break-words text-xs text-muted-foreground">
          {!row.model
            ? "Actual model unavailable"
            : row.model_alias === row.model
              ? "No distinct alias observed · Direct or canonicalized"
              : `Observed remap → ${row.model}`}
        </p>
        <p className="mt-1 break-words text-xs text-muted-foreground">{row.provider || "Provider unavailable"}</p>
      </div>
      <div className="grid w-full grid-cols-2 gap-x-4 gap-y-1 text-xs sm:w-auto sm:shrink-0 sm:text-right">
        <Metric label="Attempts" value={formatCompact(row.attempt_count)} />
        <Metric label="Failures" value={formatPercent(row.failure_share)} />
        <Metric label="Mean latency" value={row.latency_sample_count > 0 ? `${row.mean_latency_ms.toLocaleString("en", { maximumFractionDigits: 1 })} ms · ${row.latency_sample_count.toLocaleString("en")} ${row.latency_sample_count === 1 ? "sample" : "samples"}` : "No samples"} />
        <Metric label="Cost" value={formatObservedCost(row.total_cost, row.cost_status)} />
      </div>
    </>
  )
  const className = "relative flex min-w-0 flex-col gap-3 rounded-lg border border-border p-3 pr-8 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
  if (!row.model || !row.provider) return <div className={className}>{content}</div>
  return (
    <Link
      to="/requests"
      search={{ provider: row.provider, model: row.model, modelAlias: row.model_alias, account: "", endpoint: "", status: "", requestId: "", windowEnd, result: "" }}
      aria-label={`Inspect ${row.model_alias} to ${row.model} attempts`}
      className={`${className} transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500`}
    >
      {content}<ArrowUpRight className="absolute right-3 top-3 h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
    </Link>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return <span><span className="block text-[10px] text-muted-foreground">{label}</span><span className="font-medium sm:whitespace-nowrap">{value}</span></span>
}

function formatObservedCost(value: number, status: CostStatus) {
  if (status === "unavailable") return "Unavailable"
  return `${formatCost(value)}${status === "partial" ? " · Partial" : ""}`
}
