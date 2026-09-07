import { describe, expect, it } from "vitest"
import { deriveIngestionObservations } from "./ingestion-observations"

describe("deriveIngestionObservations", () => {
  it("projects populated observations without treating them as source freshness", () => {
    expect(deriveIngestionObservations({
      redis_inbox_pending: 3,
      redis_events_last_processed_at: "2026-09-07T01:02:03Z",
      redis_events_processing_rate_per_minute: 12.5,
      redis_events_processed_total: 13,
      redis_events_processed_batches_total: 2,
      poller_running: true,
      poller_sync_running: true,
    })).toEqual({
      backlog: { label: "3", detail: "Retryable rows pending in the local inbox." },
      lastProcessed: { label: "2026-09-07T01:02:03Z", observedAt: "2026-09-07T01:02:03Z", detail: "Last observed nonempty local processing batch." },
      processingRate: { label: "12.5 events/min", detail: "Observed local processing between metric scrapes. 13 events in 2 nonempty batches processed in this process." },
      runtime: { label: "Processing active", detail: "The active local runner is pulling, processing, or serving a manual sync." },
    })
  })

  it("keeps a measured zero rate distinct from the first scrape", () => {
    expect(deriveIngestionObservations({
      redis_events_processing_rate_per_minute: 0,
      redis_events_processed_total: 0,
      redis_events_processed_batches_total: 0,
    }).processingRate).toEqual({
      label: "0 events/min",
      detail: "Observed local processing between metric scrapes. 0 events in 0 nonempty batches processed in this process.",
    })
    expect(deriveIngestionObservations({}).processingRate).toEqual({
      label: "Rate unavailable",
      detail: "A processing rate is available after a comparable metrics scrape. Processed volume is not observed for this process.",
    })
  })

  it("keeps runner lifecycle and active processing state distinct", () => {
    expect(deriveIngestionObservations({ poller_running: true, poller_sync_running: false }).runtime.label).toBe("Runner idle")
    expect(deriveIngestionObservations({ poller_running: true, poller_sync_running: true }).runtime.label).toBe("Processing active")
    expect(deriveIngestionObservations({ poller_running: false }).runtime.label).toBe("Runner stopped")
    expect(deriveIngestionObservations({}).runtime.label).toBe("Runtime unavailable")
  })

  it("keeps missing timestamp and database degradation distinct from an observed inbox count", () => {
    const observations = deriveIngestionObservations({
      poller_running: true,
      poller_sync_running: false,
      db_unavailable: true,
      redis_inbox_pending: 4,
    })

    expect(observations.backlog).toEqual({
      label: "4",
      detail: "Retryable rows pending in the local inbox. Other database-backed observations are unavailable.",
    })
    expect(observations.lastProcessed.label).toBe("Not observed")
    expect(observations.runtime.label).toBe("Runner idle")
  })

  it("shows an unavailable inbox only when its field is absent during database degradation", () => {
    expect(deriveIngestionObservations({ db_unavailable: true }).backlog.label).toBe("Unavailable")
    expect(deriveIngestionObservations({}).backlog.label).toBe("Not observed")
  })
})
