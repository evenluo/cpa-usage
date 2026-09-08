import { Link } from "@tanstack/react-router"
import { ArrowUpRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { formatLatency, formatOutputTPS } from "@/components/intelligence/request-evidence-event"
import { formatCompact } from "@/lib/format"
import type {
  UsageAttemptPerformance,
  UsageAttemptPerformanceSummary,
  UsagePercentileDistribution,
  UsagePerformanceBreakdown,
} from "@/types/api"

interface AttemptPerformanceProps {
  provider: string
  data: UsageAttemptPerformance | undefined
  isLoading: boolean
  error: unknown
  onRetry: () => void
}

export function AttemptPerformance({ provider, data, isLoading, error, onRetry }: AttemptPerformanceProps) {
  const hasCompleteData = data !== undefined
  return (
    <Card className="min-w-0 overflow-hidden">
      <CardHeader className="flex flex-col items-start justify-between gap-3 sm:flex-row">
        <div>
          <CardTitle>Attempt performance</CardTitle>
        </div>
        {hasCompleteData && error ? <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry refresh</Button> : null}
      </CardHeader>
      <CardContent>
        {!hasCompleteData && isLoading ? (
          <Skeleton className="h-52 w-full" />
        ) : !hasCompleteData && error ? (
          <div className="flex h-52 flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
            <span>Failed to load attempt performance</span>
            <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry attempt performance</Button>
          </div>
        ) : !data || data.total_attempts === 0 ? (
          <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No attempts in the last 24 hours
          </div>
        ) : (
          <div className="space-y-5">
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
              <span className="font-serif text-2xl font-semibold">{formatCompact(data.total_attempts)}</span>
              <span className="text-xs text-muted-foreground">
                attempts · {formatCompact(data.successful_attempts)} successful · {formatCompact(data.failed_attempts)} failed
              </span>
            </div>
            <PerformanceSummary summary={data} provider={provider} windowEnd={data.window_end} />
            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer rounded-sm font-medium focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">Sample rules</summary>
              <p className="mt-2">
                TTFT and Output TPS exclude failed attempts plus {formatCompact(data.successful_execution.non_generating)} non-generating and {formatCompact(data.successful_execution.non_streaming)} non-streaming successful attempts. Output TPS additionally requires complete canonical output and valid timing.
              </p>
            </details>
            <details className="rounded-lg border border-border p-3">
              <summary className="cursor-pointer text-sm font-medium">Breakdown</summary>
              <div className="mt-4 grid gap-5 xl:grid-cols-3">
                <PerformanceBreakdown title="Providers" breakdown={data.providers} provider={provider} windowEnd={data.window_end} selection="provider" />
                <PerformanceBreakdown title="Actual models" breakdown={data.models} provider={provider} windowEnd={data.window_end} selection="model" />
                <PerformanceBreakdown title="Accounts" breakdown={data.accounts} provider={provider} windowEnd={data.window_end} selection="account" />
              </div>
            </details>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function PerformanceSummary({ summary, provider, windowEnd }: { summary: UsageAttemptPerformanceSummary; provider: string; windowEnd: string }) {
  return (
    <div className="grid gap-4 md:grid-cols-3">
      <MetricSection title="Latency">
        <MetricRow label="Successful" metric={summary.latency_ms.successful} kind="latency" slowLink={{ provider, windowEnd, result: "success" }} />
        <MetricRow label="Failed" metric={summary.latency_ms.failed} kind="latency" slowLink={{ provider, windowEnd, result: "failed" }} />
      </MetricSection>
      <MetricSection title="TTFT">
        <MetricRow label="Generate + stream" metric={summary.ttft_ms.generating_streaming} kind="latency" />
        <MetricRow label="Execution unknown" metric={summary.ttft_ms.unknown_execution} kind="latency" />
      </MetricSection>
      <MetricSection title="Output TPS">
        {provider ? (
          <MetricRow label="Canonical generate + stream" metric={summary.output_tps.generating_streaming} kind="tps" />
        ) : (
          <p className="text-xs text-muted-foreground">Select a provider for comparable throughput.</p>
        )}
      </MetricSection>
    </div>
  )
}

function MetricSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="min-w-0 rounded-lg bg-muted/40 p-3" aria-label={title}>
      <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
      <div className="mt-2 space-y-3">{children}</div>
    </section>
  )
}

function MetricRow({
  label,
  metric,
  kind,
  slowLink,
}: {
  label: string
  metric: UsagePercentileDistribution
  kind: "latency" | "tps"
  slowLink?: { provider: string; windowEnd: string; result: "success" | "failed"; model?: string; account?: string }
}) {
  const coverage = formatSampleCoverage(metric)
  return (
    <div className="min-w-0">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="font-medium">{label}</span>
        <span className="text-muted-foreground" title={coverage.title}>{coverage.text}</span>
      </div>
      <div className="mt-1 grid grid-cols-2 gap-2 text-xs">
        <span><span className="text-muted-foreground">p50 </span>{formatMetricValue(metric.p50, kind)}</span>
        {slowLink && metric.p95 !== null ? (
          <Link
            to="/requests"
            search={{
              provider: slowLink.provider,
              model: slowLink.model || "",
              modelAlias: "",
              account: slowLink.account || "",
              endpoint: "",
              status: "",
              requestId: "",
              minLatencyMS: String(Math.ceil(metric.p95)),
              windowEnd: slowLink.windowEnd,
              result: slowLink.result,
            }}
            className="inline-flex items-center justify-end gap-1 font-medium text-terracotta-700 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-terracotta-500 dark:text-terracotta-300"
            aria-label={`Inspect ${label.toLowerCase()} attempts at or above p95 latency`}
          >
            <span><span className="text-muted-foreground">p95 </span>{formatMetricValue(metric.p95, kind)}</span>
            <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
          </Link>
        ) : (
          <span className="text-right"><span className="text-muted-foreground">p95 </span>{formatMetricValue(metric.p95, kind)}</span>
        )}
      </div>
    </div>
  )
}

function PerformanceBreakdown({
  title,
  breakdown,
  provider,
  windowEnd,
  selection,
}: {
  title: string
  breakdown: UsagePerformanceBreakdown
  provider: string
  windowEnd: string
  selection: "provider" | "model" | "account"
}) {
  return (
    <section className="min-w-0" aria-label={title}>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
      <div className="space-y-2">
        {breakdown.items.map((item) => (
          <div key={item.value} className="rounded-md border border-border p-2 text-xs">
            <div className="flex min-w-0 items-center justify-between gap-2">
              <span className="truncate font-medium">{item.label}</span>
              <span className="shrink-0 text-muted-foreground">{formatCompact(item.attempt_count)} attempts</span>
            </div>
            <div className="mt-1 grid grid-cols-3 gap-2 text-muted-foreground">
              <span>Latency {formatPair(item.latency_ms.successful, "latency")}</span>
              <span>TTFT {formatPair(item.ttft_ms.generating_streaming, "latency")}</span>
              <span>TPS {selection === "provider" || provider ? formatPair(item.output_tps.generating_streaming, "tps") : "Select provider"}</span>
            </div>
            {item.latency_ms.successful.p95 !== null ? (
              <Link
                to="/requests"
                search={{
                  provider: selection === "provider" ? item.value : provider,
                  model: selection === "model" ? item.value : "",
                  modelAlias: "",
                  account: selection === "account" ? item.value : "",
                  endpoint: "",
                  status: "",
                  requestId: "",
                  minLatencyMS: String(Math.ceil(item.latency_ms.successful.p95)),
                  windowEnd,
                  result: "success",
                }}
                className="mt-2 inline-flex items-center gap-1 font-medium text-terracotta-700 hover:underline dark:text-terracotta-300"
              >
                Inspect slow successes <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
              </Link>
            ) : null}
          </div>
        ))}
        {breakdown.other_count > 0 ? (
          <div className="flex justify-between gap-2 px-2 py-1 text-xs text-muted-foreground">
            <span>Other or unavailable</span><span>{formatCompact(breakdown.other_count)} attempts</span>
          </div>
        ) : null}
      </div>
    </section>
  )
}

function formatPair(metric: UsagePercentileDistribution, kind: "latency" | "tps") {
  return `${formatMetricValue(metric.p50, kind)} / ${formatMetricValue(metric.p95, kind)} (${metric.sample_count}/${metric.population_count})`
}

function formatMetricValue(value: number | null, kind: "latency" | "tps") {
  if (value === null) return "-"
  return kind === "latency" ? formatLatency(value) : formatOutputTPS(value)
}

function formatSampleCoverage(metric: UsagePercentileDistribution): { text: string; title: string } {
  const counts = `${metric.sample_count.toLocaleString("en")} of ${metric.population_count.toLocaleString("en")} attempts sampled`
  if (metric.coverage === null) return { text: "—", title: `${counts}; coverage unavailable` }
  return { text: `${(metric.coverage * 100).toFixed(0)}%`, title: counts }
}
