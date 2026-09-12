import type { MetricsPayload } from "@/types/api"

export interface IngestionObservations {
  backlog: { label: string; detail: string }
  lastProcessed: { label: string; detail: string; observedAt?: string }
  processingRate: { label: string; detail: string }
  runtime: { label: string; detail: string }
}

function countLabel(value: number | undefined): string {
  return value === undefined ? "Not observed" : value.toLocaleString()
}

function processedVolumeDetail(metrics: MetricsPayload): string {
  if (metrics.redis_events_processed_total === undefined) {
    return "No volume yet."
  }
  const events = metrics.redis_events_processed_total.toLocaleString()
  if (metrics.redis_events_processed_batches_total === undefined) {
    return `${events} events.`
  }
  return `${events} events in ${metrics.redis_events_processed_batches_total.toLocaleString()} batches.`
}

export function deriveIngestionObservations(metrics: MetricsPayload): IngestionObservations {
  const dbUnavailable = metrics.db_unavailable === true
  const rate = metrics.redis_events_processing_rate_per_minute

  return {
    backlog: metrics.redis_inbox_pending !== undefined
      ? {
          label: countLabel(metrics.redis_inbox_pending),
          detail: dbUnavailable
            ? "Retryable rows pending in the local inbox. Other database-backed observations are unavailable."
            : "Retryable rows pending in the local inbox.",
        }
      : dbUnavailable
        ? { label: "Unavailable", detail: "The local inbox reading is unavailable because the database could not be read." }
        : { label: "Not observed", detail: "No local inbox reading was observed." },
    lastProcessed: metrics.redis_events_last_processed_at
      ? { label: metrics.redis_events_last_processed_at, observedAt: metrics.redis_events_last_processed_at, detail: "Last nonempty batch this process handled." }
      : { label: "Not observed", detail: "No nonempty local processing batch has been observed in this process." },
    processingRate: rate === undefined
      ? { label: "Rate unavailable", detail: `Need two scrapes to compute a rate. ${processedVolumeDetail(metrics)}` }
      : { label: `${rate.toLocaleString(undefined, { maximumFractionDigits: 1 })} events/min`, detail: `${processedVolumeDetail(metrics)}` },
    runtime: metrics.poller_running !== true
      ? metrics.poller_running === false
        ? { label: "Runner stopped", detail: "Poller is stopped." }
        : { label: "Runtime unavailable", detail: "No local poller runtime state was observed." }
      : metrics.poller_sync_running === true
        ? { label: "Processing active", detail: "Pulling or syncing." }
        : { label: "Runner idle", detail: "Idle." },
  }
}
