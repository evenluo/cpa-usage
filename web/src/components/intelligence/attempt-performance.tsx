import { Link } from "@tanstack/react-router"
import { ArrowUpRight } from "lucide-react"
import { useState } from "react"
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

type BreakdownMetric = "successful-latency" | "failed-latency" | "ttft" | "output-tps"

const breakdownMetrics: Array<{ value: BreakdownMetric; label: string }> = [
  { value: "successful-latency", label: "Successful latency" },
  { value: "failed-latency", label: "Failed latency" },
  { value: "ttft", label: "TTFT" },
  { value: "output-tps", label: "Output TPS" },
]

export function AttemptPerformance({ provider, data, isLoading, error, onRetry }: AttemptPerformanceProps) {
  const hasCompleteData = data !== undefined
  const [breakdownMetric, setBreakdownMetric] = useState<BreakdownMetric>("successful-latency")
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
              <div className="mt-4 grid grid-cols-2 gap-1 rounded-lg border border-border bg-card p-1 sm:flex sm:flex-wrap" aria-label="Performance breakdown metric">
                {breakdownMetrics.map((metric) => (
                  <button
                    key={metric.value}
                    type="button"
                    onClick={() => setBreakdownMetric(metric.value)}
                    aria-pressed={breakdownMetric === metric.value}
                    className={`min-h-10 min-w-0 rounded-md px-2 py-1.5 text-xs font-medium transition-colors sm:min-h-0 sm:px-3 ${
                      breakdownMetric === metric.value
                        ? "bg-terracotta-500 text-white"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground"
                    }`}
                  >
                    {metric.label}
                  </button>
                ))}
              </div>
              <div className="mt-4 grid gap-5 xl:grid-cols-3">
                <PerformanceBreakdown title="Providers" breakdown={data.providers} provider={provider} windowEnd={data.window_end} selection="provider" metric={breakdownMetric} />
                <PerformanceBreakdown title="Actual models" breakdown={data.models} provider={provider} windowEnd={data.window_end} selection="model" metric={breakdownMetric} />
                <PerformanceBreakdown title="Accounts" breakdown={data.accounts} provider={provider} windowEnd={data.window_end} selection="account" metric={breakdownMetric} />
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
  metric,
}: {
  title: string
  breakdown: UsagePerformanceBreakdown
  provider: string
  windowEnd: string
  selection: "provider" | "model" | "account"
  metric: BreakdownMetric
}) {
  const metricKind = metric === "output-tps" ? "tps" : "latency"
  const metrics = breakdown.items.map((item) => getBreakdownMetric(item, metric))
  const axisMaximum = metrics.reduce((maximum, distribution) => Math.max(maximum, distribution.p95 ?? 0), 0)
  const showChart = metric !== "output-tps" || Boolean(provider)
  const needsProvider = metric === "output-tps" && !provider && selection !== "provider"

  return (
    <section className="min-w-0" aria-label={title}>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
      {needsProvider ? (
        <div className="flex min-h-24 items-center justify-center rounded-md border border-dashed border-border px-3 text-center text-xs text-muted-foreground">
          Select a provider to compare {selection === "model" ? "actual models" : "accounts"}.
        </div>
      ) : breakdown.items.length === 0 ? (
        <div className="flex min-h-24 items-center justify-center rounded-md border border-dashed border-border px-3 text-center text-xs text-muted-foreground">
          No groups available
        </div>
      ) : (
      <div className="space-y-2">
        {showChart ? <PerformanceAxis maximum={axisMaximum} kind={metricKind} /> : (
          <p className="px-1 text-[11px] leading-4 text-muted-foreground">Provider throughput is shown as qualified values; select one provider for a shared comparison axis.</p>
        )}
        {breakdown.items.map((item) => {
          const distribution = getBreakdownMetric(item, metric)
          return (
            <PerformanceRow
              key={item.value}
              item={item}
              distribution={distribution}
              kind={metricKind}
              axisMaximum={axisMaximum}
              showChart={showChart}
              slowLink={metric === "successful-latency" || metric === "failed-latency" ? {
                provider: selection === "provider" ? item.value : provider,
                model: selection === "model" ? item.value : "",
                account: selection === "account" ? item.value : "",
                result: metric === "successful-latency" ? "success" : "failed",
                windowEnd,
              } : undefined}
            />
          )
        })}
      </div>
      )}
      {breakdown.other_count > 0 ? (
        <div className="mt-2 flex justify-between gap-2 px-2 py-1 text-xs text-muted-foreground">
          <span>Other or unavailable</span><span>{formatCompact(breakdown.other_count)} attempts</span>
        </div>
      ) : null}
    </section>
  )
}

function PerformanceAxis({ maximum, kind }: { maximum: number; kind: "latency" | "tps" }) {
  return (
    <div className="flex items-center justify-between gap-2 px-1 text-[10px] tabular-nums text-muted-foreground" aria-label={`Shared linear axis from zero to ${formatAxisValue(maximum, kind)}`}>
      <span>{kind === "latency" ? "0s" : "0 tok/s"}</span>
      <span className="flex items-center gap-1.5" aria-hidden="true">
        <span className="inline-block h-2 w-2 rounded-full border border-slate-600 dark:border-slate-300" /> p50
        <span className="inline-block h-2 w-2 rotate-45 border border-terracotta-600 dark:border-terracotta-300" /> p95
      </span>
      <span>linear · max p95 {formatAxisValue(maximum, kind)}</span>
    </div>
  )
}

function PerformanceRow({
  item,
  distribution,
  kind,
  axisMaximum,
  showChart,
  slowLink,
}: {
  item: UsagePerformanceBreakdown["items"][number]
  distribution: UsagePercentileDistribution
  kind: "latency" | "tps"
  axisMaximum: number
  showChart: boolean
  slowLink?: { provider: string; model: string; account: string; result: "success" | "failed"; windowEnd: string }
}) {
  const coverage = formatSampleCoverage(distribution)
  const p50Position = percentilePosition(distribution.p50, axisMaximum)
  const p95Position = percentilePosition(distribution.p95, axisMaximum)
  return (
    <div className="rounded-md border border-border p-2.5 text-xs">
      <div className="flex min-w-0 items-start justify-between gap-2">
        <span className="min-w-0 break-words font-medium [overflow-wrap:anywhere]">{item.label}</span>
        <span className="shrink-0 text-muted-foreground">{formatCompact(item.attempt_count)} attempts</span>
      </div>
      {showChart ? (
        distribution.p50 === null && distribution.p95 === null ? (
          <div className="mt-3 flex h-4 items-center justify-center rounded bg-muted/50 text-[10px] text-muted-foreground">No valid samples</div>
        ) : (
          <div
            className="relative mt-3 h-4"
            role="img"
            aria-label={`${item.label}: p50 ${formatMetricValue(distribution.p50, kind)}, p95 ${formatMetricValue(distribution.p95, kind)}`}
          >
            <div className="absolute top-1/2 h-px w-full -translate-y-1/2 bg-border" />
            {p50Position !== null && p95Position !== null ? (
              <div
                className="absolute top-1/2 h-0.5 -translate-y-1/2 bg-slate-400 dark:bg-slate-500"
                style={{ left: `${Math.min(p50Position, p95Position)}%`, width: `${Math.abs(p95Position - p50Position)}%` }}
              />
            ) : null}
            {p50Position !== null ? (
              <span data-percentile="p50" className="absolute top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-slate-600 bg-background dark:border-slate-300" style={{ left: `${p50Position}%` }} aria-hidden="true" />
            ) : null}
            {p95Position !== null ? (
              <span data-percentile="p95" className="absolute top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rotate-45 border-2 border-terracotta-600 bg-background dark:border-terracotta-300" style={{ left: `${p95Position}%` }} aria-hidden="true" />
            ) : null}
          </div>
        )
      ) : null}
      <div className="mt-2 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 tabular-nums">
        <span><span className="text-muted-foreground">p50 </span>{formatMetricValue(distribution.p50, kind)}</span>
        {slowLink && distribution.p95 !== null ? (
          <Link
            to="/requests"
            search={{
              provider: slowLink.provider,
              model: slowLink.model,
              modelAlias: "",
              account: slowLink.account,
              endpoint: "",
              status: "",
              requestId: "",
              minLatencyMS: String(Math.ceil(distribution.p95)),
              windowEnd: slowLink.windowEnd,
              result: slowLink.result,
            }}
            className="inline-flex items-center gap-1 font-medium text-terracotta-700 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-terracotta-500 dark:text-terracotta-300"
            aria-label={`Inspect ${item.label} ${slowLink.result} attempts at or above p95 latency`}
          >
            <span><span className="text-muted-foreground">p95 </span>{formatMetricValue(distribution.p95, kind)}</span>
            <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
          </Link>
        ) : (
          <span><span className="text-muted-foreground">p95 </span>{formatMetricValue(distribution.p95, kind)}</span>
        )}
      </div>
      <div className="mt-1 flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
        <span>{distribution.sample_count.toLocaleString("en")} / {distribution.population_count.toLocaleString("en")} samples</span>
        <span title={coverage.title}>{coverage.text} coverage</span>
      </div>
    </div>
  )
}

function getBreakdownMetric(item: UsageAttemptPerformanceSummary, metric: BreakdownMetric): UsagePercentileDistribution {
  if (metric === "successful-latency") return item.latency_ms.successful
  if (metric === "failed-latency") return item.latency_ms.failed
  if (metric === "ttft") return item.ttft_ms.generating_streaming
  return item.output_tps.generating_streaming
}

function percentilePosition(value: number | null, maximum: number): number | null {
  if (value === null || maximum <= 0) return null
  return Math.max(0, Math.min(100, (value / maximum) * 100))
}

function formatAxisValue(value: number, kind: "latency" | "tps") {
  if (value <= 0) return kind === "latency" ? "0s" : "0 tok/s"
  return formatMetricValue(value, kind)
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
