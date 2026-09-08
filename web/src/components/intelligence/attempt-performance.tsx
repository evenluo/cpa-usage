import { Link } from "@tanstack/react-router"
import * as Popover from "@radix-ui/react-popover"
import * as Select from "@radix-ui/react-select"
import { ArrowUpRight, Info, Check, ChevronDown, ChevronUp } from "lucide-react"
import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { formatLatency, formatOutputTPS } from "@/components/intelligence/request-evidence-event"
import { formatCompact } from "@/lib/format"
import type { UsageAttemptPerformance, UsageAttemptPerformanceSummary, UsagePercentileDistribution, UsagePerformanceBreakdown } from "@/types/api"

interface AttemptPerformanceProps {
  provider: string
  providers: string[]
  providersError?: unknown
  onRetryProviders: () => void
  onSelectProvider: (provider: string) => void
  data: UsageAttemptPerformance | undefined
  isLoading: boolean
  error: unknown
  onRetry: () => void
}

type PerformanceMetric = "successful-latency" | "failed-latency" | "ttft" | "output-tps"
type PrimaryMetric = Exclude<PerformanceMetric, "failed-latency">
type ComparisonDimension = "model" | "provider" | "account"

const performanceMetrics: Array<{ value: PrimaryMetric; label: string }> = [
  { value: "successful-latency", label: "Successful latency" },
  { value: "ttft", label: "TTFT" },
  { value: "output-tps", label: "Output TPS" },
]

const comparisonDimensions: Array<{ value: ComparisonDimension; label: string }> = [
  { value: "model", label: "Actual models" },
  { value: "provider", label: "Providers" },
  { value: "account", label: "Accounts" },
]

export function AttemptPerformance({ provider, providers, providersError, onRetryProviders, onSelectProvider, data, isLoading, error, onRetry }: AttemptPerformanceProps) {
  const hasCompleteData = data !== undefined
  const [metric, setMetric] = useState<PrimaryMetric>("successful-latency")
  const [dimension, setDimension] = useState<ComparisonDimension>("model")

  return (
    <Card className="min-w-0 overflow-hidden">
      <CardHeader className="flex flex-col items-start justify-between gap-3 pb-4 *:not-first:mt-0 sm:flex-row sm:items-center">
        <div>
          <CardTitle>Attempt performance</CardTitle>
          <p className="mt-1 text-xs text-muted-foreground">Last 24h</p>
        </div>
        <div className="w-full sm:w-auto">
          <div className="flex items-center gap-3">
            <span className="shrink-0 text-xs text-muted-foreground">Provider</span>
            <Select.Root value={provider} onValueChange={onSelectProvider} disabled={providers.length === 0}>
              <Select.Trigger aria-label="Performance provider" className="flex min-h-10 min-w-0 flex-1 items-center justify-between sm:w-48 sm:flex-none gap-2 rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground outline-hidden transition-colors hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-60 data-[state=open]:border-terracotta-500">
                <Select.Value placeholder={isLoading ? "Loading providers…" : error ? "Providers unavailable" : "No providers"} />
                <Select.Icon><ChevronDown className="h-4 w-4 text-muted-foreground" /></Select.Icon>
              </Select.Trigger>
              <Select.Portal>
                <Select.Content position="popper" sideOffset={6} collisionPadding={12} className="z-50 max-h-[min(20rem,var(--radix-select-content-available-height))] min-w-[var(--radix-select-trigger-width)] overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-lg">
                  <Select.ScrollUpButton className="flex justify-center py-1"><ChevronUp className="h-4 w-4" /></Select.ScrollUpButton>
                  <Select.Viewport className="p-1">
                    {providers.map((value) => (
                      <Select.Item key={value} value={value} className="relative flex min-h-9 cursor-default select-none items-center rounded-md py-2 pr-8 pl-3 text-sm outline-hidden data-highlighted:bg-muted data-[state=checked]:bg-terracotta-500/10 data-[state=checked]:text-terracotta-700 dark:data-[state=checked]:text-terracotta-300">
                        <Select.ItemText>{value}</Select.ItemText>
                        <Select.ItemIndicator className="absolute right-2"><Check className="h-4 w-4" /></Select.ItemIndicator>
                      </Select.Item>
                    ))}
                  </Select.Viewport>
                  <Select.ScrollDownButton className="flex justify-center py-1"><ChevronDown className="h-4 w-4" /></Select.ScrollDownButton>
                </Select.Content>
              </Select.Portal>
            </Select.Root>
          </div>
          {providersError && providers.length > 0 ? <div className="mt-1 text-xs text-red-500">Provider list refresh failed. <button type="button" onClick={onRetryProviders} className="underline">Retry provider list</button></div> : null}
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
          <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">{provider ? "No attempts for this provider in the last 24 hours" : "No providers with attempts in the last 24 hours"}</div>
        ) : (
          <div className="space-y-3">
            <div className="flex flex-col items-start justify-between gap-x-6 gap-y-2 border-b lg:flex-row lg:items-baseline border-border pb-3">
              <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                <span className="font-serif text-2xl font-semibold">{formatCompact(data.total_attempts)}</span>
                <span className="text-xs text-muted-foreground">attempts · {formatCompact(data.failed_attempts)} failed</span>
              </div>
              {metric !== "output-tps" || provider ? (
                <div>
                  <AggregateReading
                    metric={getPerformanceMetric(data, metric)}
                    kind={metric === "output-tps" ? "tps" : "latency"}
                    slowLink={metric === "successful-latency" ? { provider, windowEnd: data.window_end, result: "success" } : undefined}
                  />
                </div>
              ) : null}
            </div>

            <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
              <ControlGroup label="Performance metric">
                {performanceMetrics.map((option) => (
                  <div key={option.value} className={`flex items-center justify-center rounded-md ${metric === option.value ? "bg-terracotta-500 text-white" : "text-muted-foreground hover:bg-muted"}`}>
                    <button
                      type="button"
                      onClick={() => setMetric(option.value)}
                      aria-pressed={metric === option.value}
                      className={`min-h-10 min-w-0 rounded-md py-1.5 text-xs font-medium transition-colors sm:min-h-0 ${option.value === "ttft" ? "pl-2 pr-1" : "flex-1 px-3"}`}
                    >
                      {option.label}
                    </button>
                    {option.value === "ttft" ? <UnknownExecutionInfo metric={data.ttft_ms.unknown_execution} /> : null}
                  </div>
                ))}
              </ControlGroup>
              <ControlGroup label="Compare by">
                {comparisonDimensions.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => setDimension(option.value)}
                    aria-pressed={dimension === option.value}
                    className={`min-h-10 min-w-0 rounded-md px-3 py-1.5 text-xs font-medium transition-colors sm:min-h-0 ${dimension === option.value ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted hover:text-foreground"}`}
                  >
                    {option.label}
                  </button>
                ))}
              </ControlGroup>
            </div>

            <PerformanceBreakdown
              title={getDimensionLabel(dimension)}
              breakdown={getDimensionBreakdown(data, dimension)}
              provider={provider}
              windowEnd={data.window_end}
              selection={dimension}
              metric={metric}
            />

            <details className="rounded-lg border border-border px-3 py-2.5">
              <summary className="cursor-pointer rounded-sm text-sm font-medium focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">Failed attempt latency · {formatCompact(data.failed_attempts)} {data.failed_attempts === 1 ? "attempt" : "attempts"}</summary>
              <div className="mt-4">
                <div className="mb-3">
                  <AggregateReading metric={data.latency_ms.failed} kind="latency" slowLink={{ provider, windowEnd: data.window_end, result: "failed" }} />
                </div>
                <PerformanceBreakdown
                  title={`${getDimensionLabel(dimension)} · failed attempts`}
                  breakdown={getDimensionBreakdown(data, dimension)}
                  provider={provider}
                  windowEnd={data.window_end}
                  selection={dimension}
                  metric="failed-latency"
                />
              </div>
            </details>

            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer rounded-sm font-medium focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">Sample rules</summary>
              <p className="mt-2">TTFT and Output TPS charts use successful streaming generation. TPS also requires complete output data and valid timing. Excluded successful attempts: {formatCompact(data.successful_execution.non_generating)} non-generating, {formatCompact(data.successful_execution.non_streaming)} non-streaming. Execution unknown: {formatCompact(data.successful_execution.unknown)}; TTFT shown separately.</p>
            </details>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ControlGroup({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1.5 text-[11px] font-medium text-muted-foreground">{label}</p>
      <div className="grid grid-cols-3 gap-1 rounded-lg border border-border bg-card p-1" aria-label={label}>{children}</div>
    </div>
  )
}

function AggregateReading({ metric, kind, slowLink }: { metric: UsagePercentileDistribution; kind: "latency" | "tps"; slowLink?: { provider: string; windowEnd: string; result: "success" | "failed" } }) {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] tabular-nums sm:gap-x-4 sm:text-xs" aria-label="Overall selected metric">
      <span><span className="text-muted-foreground">Overall p50 </span>{formatMetricValue(metric.p50, kind)}</span>
      {slowLink && metric.p95 !== null ? (
        <Link
          to="/requests"
          search={requestSearch({ ...slowLink, model: "", account: "", p95: metric.p95 })}
          className="inline-flex items-center gap-1 font-medium text-terracotta-700 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-terracotta-500 dark:text-terracotta-300"
          aria-label={`Inspect ${slowLink.result === "success" ? "successful" : "failed"} attempts at or above p95 latency`}
        >
          <span><span className="text-muted-foreground">p95 </span>{formatMetricValue(metric.p95, kind)}</span><ArrowUpRight className="h-3 w-3" aria-hidden="true" />
        </Link>
      ) : (
        <span><span className="text-muted-foreground">p95 </span>{formatMetricValue(metric.p95, kind)}</span>
      )}
      <SampleCoverage metric={metric} label="Overall" />
    </div>
  )
}

function UnknownExecutionInfo({ metric }: { metric: UsagePercentileDistribution }) {
  const coverage = formatSampleCoverage(metric)
  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <button type="button" aria-label="About TTFT execution unknown" className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-sm opacity-75 hover:opacity-100 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">
          <Info className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content sideOffset={8} collisionPadding={12} aria-label="Unknown execution TTFT" className="z-50 w-72 max-w-[calc(100vw-24px)] rounded-lg border border-border bg-popover p-3 text-xs text-popover-foreground shadow-lg">
          <p className="font-medium">Execution unknown</p>
          <p className="mt-1 text-muted-foreground">Attempts with unknown execution type are shown separately from the successful streaming generation used for Overall TTFT.</p>
          <div className="mt-3 flex gap-4 tabular-nums">
            <span>p50 {formatMetricValue(metric.p50, "latency")}</span>
            <span>p95 {formatMetricValue(metric.p95, "latency")}</span>
          </div>
          <p className="mt-2 text-muted-foreground" title={coverage.title}>{metric.sample_count.toLocaleString("en")} / {metric.population_count.toLocaleString("en")} samples · {metric.coverage === null ? "Coverage unavailable" : `${coverage.text} coverage`}</p>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

function PerformanceBreakdown({ title, breakdown, provider, windowEnd, selection, metric }: {
  title: string
  breakdown: UsagePerformanceBreakdown
  provider: string
  windowEnd: string
  selection: ComparisonDimension
  metric: PerformanceMetric
}) {
  const metricKind = metric === "output-tps" ? "tps" : "latency"
  const metrics = breakdown.items.map((item) => getPerformanceMetric(item, metric))
  const axisMaximum = metrics.reduce((maximum, distribution) => Math.max(maximum, distribution.histogram?.upper_bound ?? 0, distribution.p95 ?? 0), 0)
  const showChart = metric !== "output-tps" || Boolean(provider)
  const needsProvider = metric === "output-tps" && !provider && selection !== "provider"

  return (
    <section className="min-w-0" aria-label={title}>
      <div className="mb-2 flex flex-wrap items-center justify-end gap-x-4 gap-y-1.5">
        {!needsProvider && showChart && metrics.some((distribution) => distribution.histogram !== null) ? <DensityLegend /> : null}
        {!needsProvider && breakdown.items.length > 0 && showChart ? <PerformanceAxis maximum={axisMaximum} kind={metricKind} /> : null}
      </div>
      {needsProvider ? (
        <div className="flex min-h-28 items-center justify-center rounded-md border border-dashed border-border px-3 text-center text-xs text-muted-foreground">Select a provider to compare output speed.</div>
      ) : breakdown.items.length === 0 ? (
        <div className="flex min-h-28 items-center justify-center rounded-md border border-dashed border-border px-3 text-center text-xs text-muted-foreground">No groups available</div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border">
          {!showChart ? <p className="border-b border-border bg-muted/30 px-3 py-2 text-[11px] leading-4 text-muted-foreground">Select a provider to compare output speed.</p> : null}
          <div className="divide-y divide-border">
            {breakdown.items.map((item) => (
              <PerformanceRow
                key={item.value}
                item={item}
                distribution={getPerformanceMetric(item, metric)}
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
            ))}
          </div>
        </div>
      )}
      {breakdown.other_count > 0 ? <div className="mt-2 flex justify-between gap-2 px-2 py-1 text-xs text-muted-foreground"><span>Other or unavailable</span><span>{formatCompact(breakdown.other_count)} attempts</span></div> : null}
    </section>
  )
}

function PerformanceAxis({ maximum, kind }: { maximum: number; kind: "latency" | "tps" }) {
  return (
    <div className="flex items-center gap-3 text-[10px] tabular-nums text-muted-foreground" aria-label={`Shared linear axis from zero to ${formatAxisValue(maximum, kind)}`}>
      <span className="flex items-center gap-1" aria-hidden="true"><span className="inline-block h-2 w-2 rounded-full border border-slate-600 dark:border-slate-300" /> p50</span>
      <span className="flex items-center gap-1" aria-hidden="true"><span className="inline-block h-2 w-2 rotate-45 border border-terracotta-600 dark:border-terracotta-300" /> p95</span>
      <span>0 to {formatAxisValue(maximum, kind)}</span>
    </div>
  )
}

function DensityLegend() {
  return (
    <div className="flex items-center gap-1.5 text-[10px] text-muted-foreground" title="Each interval shows its share of this row's valid samples. The same color scale applies to every row.">
      <span className="flex overflow-hidden rounded-sm" aria-hidden="true">
        {[0.05, 0.2, 0.5, 1].map((share) => <span key={share} className="h-1.5 w-2.5 bg-sky-700 dark:bg-sky-400" style={{ opacity: densityOpacity(share) }} />)}
      </span>
      <span>Stronger color = higher sample share</span>
    </div>
  )
}

function PerformanceRow({ item, distribution, kind, axisMaximum, showChart, slowLink }: {
  item: UsagePerformanceBreakdown["items"][number]
  distribution: UsagePercentileDistribution
  kind: "latency" | "tps"
  axisMaximum: number
  showChart: boolean
  slowLink?: { provider: string; model: string; account: string; result: "success" | "failed"; windowEnd: string }
}) {
  const p50Position = percentilePosition(distribution.p50, axisMaximum)
  const p95Position = percentilePosition(distribution.p95, axisMaximum)
  return (
    <div className="grid min-w-0 gap-3 px-3 py-3 text-xs sm:grid-cols-[minmax(9rem,0.32fr)_minmax(0,1fr)_11rem] sm:items-center">
      <div className="min-w-0">
        <p className="break-words font-medium [overflow-wrap:anywhere]">{item.label}</p>
        <div className="mt-0.5 text-[11px] text-muted-foreground"><SampleCoverage metric={distribution} label={item.label} attempts={item.attempt_count} /></div>
      </div>
      {showChart ? (
        distribution.p50 === null && distribution.p95 === null ? (
          <div className="flex h-5 items-center justify-center rounded bg-muted/50 text-[10px] text-muted-foreground">No valid samples</div>
        ) : (
          <DistributionTrack label={item.label} distribution={distribution} kind={kind} axisMaximum={axisMaximum} p50Position={p50Position} p95Position={p95Position} />
        )
      ) : <div />}
      <div className="flex min-w-36 items-center justify-between gap-3 tabular-nums sm:justify-end">
        <span><span className="text-muted-foreground">p50 </span>{formatMetricValue(distribution.p50, kind)}</span>
        {slowLink && distribution.p95 !== null ? (
          <Link
            to="/requests"
            search={requestSearch({ ...slowLink, p95: distribution.p95 })}
            className="inline-flex items-center gap-1 font-medium text-terracotta-700 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-terracotta-500 dark:text-terracotta-300"
            aria-label={`Inspect ${item.label} ${slowLink.result} attempts at or above p95 latency`}
          >
            <span><span className="text-muted-foreground">p95 </span>{formatMetricValue(distribution.p95, kind)}</span><ArrowUpRight className="h-3 w-3" aria-hidden="true" />
          </Link>
        ) : <span><span className="text-muted-foreground">p95 </span>{formatMetricValue(distribution.p95, kind)}</span>}
      </div>
    </div>
  )
}

function DistributionTrack({ label, distribution, kind, axisMaximum, p50Position, p95Position }: {
  label: string
  distribution: UsagePercentileDistribution
  kind: "latency" | "tps"
  axisMaximum: number
  p50Position: number | null
  p95Position: number | null
}) {
  const histogram = distribution.histogram
  const sampleCount = distribution.sample_count
  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <button type="button" aria-label={`View sample distribution for ${label}`} disabled={!histogram} className="mx-1.5 block min-w-0 rounded-sm text-left focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">
          <span className="relative block h-7" role="img" aria-label={`${label}: p50 ${formatMetricValue(distribution.p50, kind)}, p95 ${formatMetricValue(distribution.p95, kind)}`}>
            <span className="absolute top-1/2 h-px w-full -translate-y-1/2 bg-border" />
            {histogram?.counts.map((count, index) => {
              const share = sampleCount > 0 ? count / sampleCount : 0
              const scale = axisMaximum > 0 ? histogram.upper_bound / axisMaximum : 1
              return <span key={index} data-heatmap-bin={index} data-count={count} data-share={share} title={`${histogramInterval(histogram, index, kind)} · ${count.toLocaleString("en")} samples · ${formatSampleShare(share)}`} className="absolute top-1/2 h-2 -translate-y-1/2 bg-sky-700 dark:bg-sky-400" style={{ left: `${index / histogram.counts.length * scale * 100}%`, width: `${scale * 100 / histogram.counts.length}%`, opacity: densityOpacity(share) }} />
            })}
            {p50Position !== null && p95Position !== null ? <span className="pointer-events-none absolute top-1/2 h-px -translate-y-1/2 bg-slate-400/70 dark:bg-slate-300/50" style={{ left: `${Math.min(p50Position, p95Position)}%`, width: `${Math.abs(p95Position - p50Position)}%` }} /> : null}
            {p50Position !== null ? <span data-percentile="p50" className="pointer-events-none absolute top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-slate-600 bg-background dark:border-slate-300" style={{ left: `${p50Position}%` }} aria-hidden="true" /> : null}
            {p95Position !== null ? <span data-percentile="p95" className="pointer-events-none absolute top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rotate-45 border-2 border-terracotta-600 bg-background dark:border-terracotta-300" style={{ left: `${p95Position}%` }} aria-hidden="true" /> : null}
          </span>
        </button>
      </Popover.Trigger>
      {histogram ? (
        <Popover.Portal>
          <Popover.Content sideOffset={8} collisionPadding={12} aria-label={`${label} sample distribution`} className="z-50 w-80 max-w-[calc(100vw-24px)] rounded-lg border border-border bg-popover p-3 text-xs text-popover-foreground shadow-lg">
            <p className="break-words font-medium">{label}</p>
            <p className="mt-1 text-muted-foreground">{sampleCount.toLocaleString("en")} valid {sampleCount === 1 ? "sample" : "samples"} · {histogram.counts.length} equal intervals</p>
            {sampleCount < 10 ? <p className="mt-1 text-muted-foreground">Few samples; the distribution may change substantially with more requests.</p> : null}
            <div className="mt-3 max-h-64 overflow-y-auto overscroll-contain">
              <table className="w-full text-left tabular-nums">
                <thead className="sticky top-0 bg-popover text-[10px] text-muted-foreground"><tr><th className="pb-2 font-normal">Interval</th><th className="pb-2 text-right font-normal">Samples</th><th className="pb-2 text-right font-normal">Share</th></tr></thead>
                <tbody>{histogram.counts.map((count, index) => <tr key={index} className="border-t border-border/50"><td className="py-1.5">{histogramInterval(histogram, index, kind)}</td><td className="py-1.5 text-right">{count.toLocaleString("en")}</td><td className="py-1.5 text-right">{formatSampleShare(sampleCount > 0 ? count / sampleCount : 0)}</td></tr>)}</tbody>
              </table>
            </div>
            <p className="mt-2 text-[10px] text-muted-foreground">Color shows share within this row. The final interval includes its upper value.</p>
          </Popover.Content>
        </Popover.Portal>
      ) : null}
    </Popover.Root>
  )
}

function densityOpacity(share: number): number {
  return share <= 0 ? 0 : 0.12 + 0.88 * Math.sqrt(share)
}

function formatSampleShare(share: number): string {
  return share > 0 && share < 0.001 ? "<0.1%" : `${(share * 100).toFixed(1)}%`
}

function histogramInterval(histogram: NonNullable<UsagePercentileDistribution["histogram"]>, index: number, kind: "latency" | "tps"): string {
  const lower = histogram.upper_bound * index / histogram.counts.length
  const upper = histogram.upper_bound * (index + 1) / histogram.counts.length
  const format = (value: number) => value.toLocaleString("en", { maximumFractionDigits: 2 })
  const unit = kind === "tps" ? "tok/s" : "ms"
  return `${format(lower)}–${format(upper)} ${unit}`
}

function requestSearch({ provider, model, account, result, windowEnd, p95 }: { provider: string; model: string; account: string; result: "success" | "failed"; windowEnd: string; p95: number }) {
  return { provider, model, modelAlias: "", account, endpoint: "", status: "", requestId: "", minLatencyMS: String(Math.ceil(p95)), windowEnd, result }
}

function getDimensionBreakdown(data: UsageAttemptPerformance, dimension: ComparisonDimension): UsagePerformanceBreakdown {
  if (dimension === "provider") return data.providers
  if (dimension === "account") return data.accounts
  return data.models
}

function getDimensionLabel(dimension: ComparisonDimension): string {
  if (dimension === "provider") return "Providers"
  if (dimension === "account") return "Accounts"
  return "Actual models"
}

function getPerformanceMetric(item: UsageAttemptPerformanceSummary, metric: PerformanceMetric): UsagePercentileDistribution {
  if (metric === "successful-latency") return item.latency_ms.successful
  if (metric === "failed-latency") return item.latency_ms.failed
  if (metric === "ttft") return item.ttft_ms.generating_streaming
  return item.output_tps.generating_streaming
}

function percentilePosition(value: number | null, maximum: number): number | null {
  if (value === null) return null
  if (maximum <= 0) return value === 0 ? 0 : null
  return Math.max(0, Math.min(100, (value / maximum) * 100))
}

function formatAxisValue(value: number, kind: "latency" | "tps") {
  if (value <= 0) return kind === "latency" ? "0s" : "0 tok/s"
  return formatMetricValue(value, kind)
}

function formatMetricValue(value: number | null, kind: "latency" | "tps") {
  if (value === null) return "-"
  if (value === 0) return kind === "latency" ? "0s" : "0.0 tok/s"
  return kind === "latency" ? formatLatency(value) : formatOutputTPS(value)
}

function SampleCoverage({ metric, label, attempts }: { metric: UsagePercentileDistribution; label: string; attempts?: number }) {
  const coverage = formatSampleCoverage(metric)
  return (
    <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-muted-foreground">
      <details>
        <summary
          aria-label={`Sample details for ${label}`}
          title="View sample details"
          className="cursor-pointer list-none rounded-sm underline decoration-dotted underline-offset-4 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring [&::-webkit-details-marker]:hidden"
        >
          {attempts === undefined ? "Samples" : `${formatCompact(attempts)} ${attempts === 1 ? "attempt" : "attempts"}`}
        </summary>
        <p className="mt-1" title={coverage.title}>{metric.sample_count.toLocaleString("en")} / {metric.population_count.toLocaleString("en")} samples · {coverage.text} coverage</p>
      </details>
      {metric.coverage !== 1 ? <span title={coverage.title}>{metric.coverage === null ? "Coverage unavailable" : `${coverage.text} coverage`}</span> : null}
    </div>
  )
}

function formatSampleCoverage(metric: UsagePercentileDistribution): { text: string; title: string } {
  const counts = `${metric.sample_count.toLocaleString("en")} of ${metric.population_count.toLocaleString("en")} attempts sampled`
  if (metric.coverage === null) return { text: "—", title: `${counts}; coverage unavailable` }
  const rounded = Math.round(metric.coverage * 100)
  const text = metric.coverage < 1 && rounded === 100 ? "<100%" : metric.coverage > 0 && rounded === 0 ? "<1%" : `${rounded}%`
  return { text, title: counts }
}
