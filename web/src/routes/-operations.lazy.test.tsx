import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"

vi.mock("@tanstack/react-router", () => ({
  createLazyFileRoute: () => (options: object) => options,
  useNavigate: () => vi.fn(),
}))

import { RollupCoverage } from "./operations.lazy"

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
