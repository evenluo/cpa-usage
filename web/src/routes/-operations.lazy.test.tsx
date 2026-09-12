import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { formatDate } from "@/lib/format"

vi.mock("@tanstack/react-router", () => ({
  createLazyFileRoute: () => (options: object) => options,
  useNavigate: () => vi.fn(),
}))

import { IngestionObservations, RollupCoverage } from "./operations.lazy"

afterEach(cleanup)

describe("Operations rollup coverage", () => {
  it("shows existing rollup observations without claiming ingestion freshness or service health", () => {
    render(<RollupCoverage status={{
      rollup_backfill: {
        status: "running",
        covered_bucket_start: "2026-08-31T00:00:00Z",
        target_bucket_start: "2026-08-31T01:00:00Z",
      },
    }} />)

    expect(screen.getByText("Rollup running")).toBeInTheDocument()
    expect(screen.getByText(/Coverage through/)).toBeInTheDocument()
    expect(screen.getByText(/Target/)).toBeInTheDocument()
    expect(screen.queryByText(/usage sync/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/healthy/i)).not.toBeInTheDocument()
  })
})

describe("Operations ingestion observations", () => {
  it("keeps manual sync separate from populated local observations", () => {
    render(<IngestionObservations
      isLoading={false}
      isError={false}
      metrics={{
        redis_inbox_pending: 2,
        redis_events_last_processed_at: "2026-09-07T01:02:03Z",
        redis_events_processing_rate_per_minute: 0,
        redis_events_processed_total: 0,
        redis_events_processed_batches_total: 0,
        poller_running: true,
        poller_sync_running: false,
      }}
    />)

    expect(screen.getByText("Ingestion observations")).toBeInTheDocument()
    expect(screen.getByText("0 events/min")).toBeInTheDocument()
    expect(screen.getByText(/0 events in 0 batches/i)).toBeInTheDocument()
    expect(screen.getByText("Runner idle")).toBeInTheDocument()
    expect(screen.getByText(formatDate("2026-09-07T01:02:03Z"))).toBeInTheDocument()
    expect(screen.getByText(/not CPA queue health/i)).toBeInTheDocument()
    expect(screen.queryByText(/last manual sync/i)).not.toBeInTheDocument()
  })

  it("makes a failed metrics request explicit", () => {
    render(<IngestionObservations isLoading={false} isError metrics={undefined} />)

    expect(screen.getByText("Ingestion observations unavailable")).toBeInTheDocument()
    expect(screen.getByText(/runtime metrics request failed/i)).toBeInTheDocument()
  })
})
