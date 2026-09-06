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
    return "Processed volume is not observed for this process."
  }
  const events = metrics.redis_events_processed_total.toLocaleString()
  if (metrics.redis_events_processed_batches_total === undefined) {
    return `${events} events processed in this process. Batch count is not observed.`
  }
  return `${events} events in ${metrics.redis_events_processed_batches_total.toLocaleString()} nonempty batches processed in this process.`
}

export function deriveIngestionObservations(metrics: MetricsPayload): IngestionObservations {
  const dbUnavailable = metrics.db_unavailable === true
  const rate = metrics.redis_events_processing_rate_per_minute

  return {
    backlog: dbUnavailable
      ? { label: "Unavailable", detail: "The local inbox reading is unavailable because the database could not be read." }
      : { label: countLabel(metrics.redis_inbox_pending), detail: "Retryable rows pending in the local inbox." },
    lastProcessed: metrics.redis_events_last_processed_at
      ? { label: metrics.redis_events_last_processed_at, observedAt: metrics.redis_events_last_processed_at, detail: "Last observed nonempty local processing batch." }
      : { label: "Not observed", detail: "No nonempty local processing batch has been observed in this process." },
    processingRate: rate === undefined
      ? { label: "Rate unavailable", detail: `A processing rate is available after a comparable metrics scrape. ${processedVolumeDetail(metrics)}` }
      : { label: `${rate.toLocaleString(undefined, { maximumFractionDigits: 1 })} events/min`, detail: `Observed local processing between metric scrapes. ${processedVolumeDetail(metrics)}` },
    runtime: metrics.poller_running === true
      ? { label: "Runner active", detail: "The local poller runner is active." }
      : metrics.poller_running === false
        ? { label: "Runner idle", detail: "The local poller runner is not active." }
        : { label: "Runtime unavailable", detail: "No local poller runtime state was observed." },
  }
}
