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
    })).toEqual({
      backlog: { label: "3", detail: "Retryable rows pending in the local inbox." },
      lastProcessed: { label: "2026-09-07T01:02:03Z", observedAt: "2026-09-07T01:02:03Z", detail: "Last observed nonempty local processing batch." },
      processingRate: { label: "12.5 events/min", detail: "Observed local processing between metric scrapes. 13 events in 2 nonempty batches processed in this process." },
      runtime: { label: "Runner active", detail: "The local poller runner is active." },
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

  it("keeps idle, missing timestamp, and unavailable database observations distinct", () => {
    const observations = deriveIngestionObservations({ poller_running: false, db_unavailable: true })

    expect(observations.backlog.label).toBe("Unavailable")
    expect(observations.lastProcessed.label).toBe("Not observed")
    expect(observations.runtime.label).toBe("Runner idle")
  })
})
