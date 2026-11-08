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
      backlog: { label: "3", detail: "Pending in inbox" },
      lastProcessed: { label: "2026-09-07T01:02:03Z", observedAt: "2026-09-07T01:02:03Z", detail: "Last processed batch" },
      processingRate: { label: "12.5 events/min", detail: "13 events in 2 batches" },
      runtime: { label: "Active", detail: "Syncing" },
    })
  })

  it("keeps a measured zero rate distinct from the first scrape", () => {
    expect(deriveIngestionObservations({
      redis_events_processing_rate_per_minute: 0,
      redis_events_processed_total: 0,
      redis_events_processed_batches_total: 0,
    }).processingRate).toEqual({
      label: "0 events/min",
      detail: "0 events in 0 batches",
    })
    expect(deriveIngestionObservations({}).processingRate).toEqual({
      label: "—",
      detail: "No volume yet",
    })
  })

  it("keeps runner lifecycle and active processing state distinct", () => {
    expect(deriveIngestionObservations({ poller_running: true, poller_sync_running: false }).runtime).toMatchObject({ label: "Idle", detail: "Waiting for next poll" })
    expect(deriveIngestionObservations({ poller_running: true, poller_sync_running: true }).runtime).toMatchObject({ label: "Active", detail: "Syncing" })
    expect(deriveIngestionObservations({ poller_running: false }).runtime).toMatchObject({ label: "Stopped", detail: "Poller stopped" })
    expect(deriveIngestionObservations({}).runtime).toMatchObject({ label: "Unavailable", detail: "State unknown" })
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
      detail: "Pending in inbox · DB unavailable",
    })
    expect(observations.lastProcessed.label).toBe("Never")
    expect(observations.runtime.label).toBe("Idle")
  })

  it("shows an unavailable inbox only when its field is absent during database degradation", () => {
    expect(deriveIngestionObservations({ db_unavailable: true }).backlog).toEqual({
      label: "Unavailable",
      detail: "Couldn't read inbox",
    })
    expect(deriveIngestionObservations({}).backlog).toEqual({
      label: "—",
      detail: "No inbox reading",
    })
  })
})
