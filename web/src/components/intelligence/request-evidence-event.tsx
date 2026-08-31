import { Badge } from "@/components/ui/badge"
import { formatCompact, formatDate } from "@/lib/format"
import type { UsageEvent } from "@/types/api"

interface RequestEvidenceEventProps {
  event: UsageEvent
  label: string
  syncState?: "synced" | "refreshing"
  detail?: boolean
}

export function RequestEvidenceEvent({ event, label, syncState, detail = false }: RequestEvidenceEventProps) {
  const { keyLabel, keyTrace } = getRequestEventLabels(event)

  return (
    <section
      aria-label={label}
      className="min-w-0 rounded-lg border border-terracotta-200 bg-terracotta-50/70 p-3 dark:border-terracotta-900/60 dark:bg-terracotta-950/20"
    >
      {syncState ? (
        <EvidenceSyncSignal state={syncState} />
      ) : (
        <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{label}</p>
      )}
      <div className="mt-1.5 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold">{keyLabel}</p>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">{keyTrace}</p>
        </div>
        <Badge variant={event.failed ? "amber" : "green"} className="shrink-0 text-[10px]">
          {event.failed ? "Failed" : "Success"}
        </Badge>
      </div>
      <p className="mt-2 truncate text-xs text-muted-foreground">
        {event.endpoint ? `${event.endpoint} · ` : ""}{event.model || "Unknown model"} · {formatDate(event.timestamp)}
      </p>
      <div className="mt-3 grid min-w-0 grid-cols-3 gap-3">
        <RequestMetric label="Output TPS" value={formatOutputTPS(event.output_tps)} />
        <RequestMetric label="Latency" value={formatLatency(event.latency_ms)} />
        <RequestMetric label="Tokens" value={formatCompact(event.tokens?.total_tokens ?? 0, 2)} />
      </div>
      {detail ? <RequestEvidenceDetail event={event} /> : null}
    </section>
  )
}

function RequestEvidenceDetail({ event }: { event: UsageEvent }) {
  const fields = [
    ["Requested model", event.model_alias || "-"],
    ["Actual model", event.model || "-"],
    ["Endpoint", event.endpoint || "-"],
    ["Request ID", event.request_id || "-"],
    ["Status code", formatOptionalNumber(event.status_code)],
    ["Executor", event.executor_type || "-"],
    ["Reasoning effort", event.reasoning_effort || "-"],
    ["Service tier", event.service_tier || "-"],
    ["TTFT", event.ttft_ms === null ? "-" : formatLatency(event.ttft_ms)],
    ["Input tokens", formatTokenCount(event.tokens?.input_tokens)],
    ["Output tokens", formatTokenCount(event.tokens?.output_tokens)],
    ["Reasoning tokens", formatTokenCount(event.tokens?.reasoning_tokens)],
    ["Generic cached tokens", formatTokenCount(event.tokens?.cached_tokens)],
    ["Cache read tokens", formatTokenCount(event.tokens?.cache_read_tokens)],
    ["Cache creation tokens", formatTokenCount(event.tokens?.cache_creation_tokens)],
  ]

  return (
    <dl className="mt-4 grid min-w-0 gap-x-4 gap-y-3 border-t border-terracotta-200 pt-3 sm:grid-cols-2 dark:border-terracotta-900/60">
      {fields.map(([label, value]) => (
        <div key={label} className="min-w-0">
          <dt className="text-[10px] text-muted-foreground">{label}</dt>
          <dd className="break-all text-xs font-medium">{value}</dd>
        </div>
      ))}
    </dl>
  )
}

function formatOptionalNumber(value: number | undefined): string {
  return typeof value === "number" && Number.isFinite(value) ? String(value) : "-"
}

function formatTokenCount(value: number | undefined): string {
  return typeof value === "number" && Number.isFinite(value) ? formatCompact(value, 2) : "-"
}

function EvidenceSyncSignal({ state }: { state: "synced" | "refreshing" }) {
  const label = state === "refreshing" ? "Syncing with trend" : "Synced with trend"

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex h-3 items-center gap-2 text-terracotta-700 dark:text-terracotta-300"
    >
      <span className="flex h-3 items-center gap-0.5" aria-hidden="true">
        <span className="h-1.5 w-0.5 origin-center rounded-full bg-current motion-safe:animate-evidence-signal motion-reduce:animate-none [animation-delay:-0.6s]" />
        <span className="h-2.5 w-0.5 origin-center rounded-full bg-current motion-safe:animate-evidence-signal motion-reduce:animate-none [animation-delay:-0.4s]" />
        <span className="h-3 w-0.5 origin-center rounded-full bg-current motion-safe:animate-evidence-signal motion-reduce:animate-none [animation-delay:-0.2s]" />
        <span className="h-2 w-0.5 origin-center rounded-full bg-current motion-safe:animate-evidence-signal motion-reduce:animate-none" />
      </span>
      <span className="text-[10px] font-medium uppercase tracking-wider">{label}</span>
    </div>
  )
}

function RequestMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="truncate text-[10px] text-muted-foreground">{label}</p>
      <p className="whitespace-nowrap text-xs font-medium">{value}</p>
    </div>
  )
}

export function getRequestEventLabels(event: UsageEvent) {
  const keyLabel = event.api_key_alias || event.api_key_display || event.source || event.auth_index || "No key trace"
  const keyTrace = [
    event.api_key_alias ? event.api_key_display : "",
    event.source || event.auth_index || "",
  ]
    .filter(Boolean)
    .join(" · ")

  return { keyLabel, keyTrace }
}

export function formatOutputTPS(value: number | null | undefined) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) return "-"
  return `${value.toFixed(1)} tok/s`
}

export function formatLatency(latencyMS: number) {
  if (!Number.isFinite(latencyMS) || latencyMS <= 0) return "-"
  const seconds = latencyMS / 1000
  return `${seconds.toLocaleString("en", { maximumFractionDigits: 2 })}s`
}
