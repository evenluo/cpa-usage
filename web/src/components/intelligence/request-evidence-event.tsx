import { Badge } from "@/components/ui/badge"
import { formatCompact, formatDate } from "@/lib/format"
import type { UsageEvent } from "@/types/api"
import { ACCOUNTING_STATE_LABELS, getCanonicalTokenFields } from "@/features/usage-intelligence/view-model"

interface RequestEvidenceEventProps {
  event: UsageEvent
  label: string
  syncState?: "synced" | "refreshing"
  detail?: boolean
}

export function RequestEvidenceEvent({ event, label, syncState, detail = false }: RequestEvidenceEventProps) {
  const { keyLabel, keyTrace } = getRequestEventLabels(event)
  const facts = event.attempt_facts

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
        <RequestMetric label="Output TPS" value={formatOutputTPS(facts.output_tps)} />
        <RequestMetric label="Latency" value={formatLatency(event.latency_ms)} />
        <RequestMetric label="Tokens" value={formatTokenCount(facts.accounting.total_tokens)} />
      </div>
      {detail ? <RequestEvidenceDetail event={event} /> : null}
    </section>
  )
}

function RequestEvidenceDetail({ event }: { event: UsageEvent }) {
  const facts = event.attempt_facts
  const accounting = facts.accounting
  const fields = [
    ["Alias", event.model_alias || "-"],
    ["Model", event.model || "-"],
    ["Endpoint", event.endpoint || "-"],
    ["Request ID", event.request_id || "-"],
    ["Status code", formatOptionalNumber(event.status_code)],
    ["Executor", event.executor_type || "-"],
    ["Reasoning effort", event.reasoning_effort || "-"],
    ["Requested service tier", facts.request_service_tier ?? "-"],
    ["Response service tier", facts.response_service_tier ?? "-"],
    ["Generate", formatOptionalBoolean(facts.generate)],
    ["Stream", formatOptionalBoolean(facts.stream)],
    ["TTFT", event.ttft_ms === null ? "-" : formatLatency(event.ttft_ms)],
    ["Token data status", ACCOUNTING_STATE_LABELS[accounting.state]],
    ["Reported quality", accounting.quality ?? "-"],
    ...getCanonicalTokenFields(accounting).map(([label, value]) => [label, formatTokenCount(value)]),
  ]

  return (
    <div className="mt-4 border-t border-terracotta-200 pt-3 dark:border-terracotta-900/60">
      <p className="mb-3 text-xs text-muted-foreground">Token counts come from the upstream provider. Missing values are shown as "-". Output TPS requires complete token data for streaming requests.</p>
      <dl className="grid min-w-0 gap-x-4 gap-y-3 sm:grid-cols-2">
        {fields.map(([label, value]) => (
          <div key={label} className="min-w-0">
            <dt className="text-[10px] text-muted-foreground">{label}</dt>
            <dd className="break-all text-xs font-medium">{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  )
}

function formatOptionalBoolean(value: boolean | null | undefined): string {
  return typeof value === "boolean" ? (value ? "Yes" : "No") : "Unknown"
}

function formatOptionalNumber(value: number | null | undefined): string {
  return typeof value === "number" && Number.isFinite(value) ? String(value) : "-"
}

function formatTokenCount(value: number | null | undefined): string {
  return typeof value === "number" && Number.isFinite(value) ? formatCompact(value, 2) : "-"
}

function EvidenceSyncSignal({ state }: { state: "synced" | "refreshing" }) {
  const label = state === "refreshing" ? "Updating" : "Live"

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
