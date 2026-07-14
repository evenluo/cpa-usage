import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it } from "vitest"
import type { UsageEvent } from "@/types/api"
import { RequestEvidenceEvent } from "./request-evidence-event"

const event: UsageEvent = {
  timestamp: "2026-07-14T12:00:00+08:00",
  model: "gpt-5",
  source: "Codex",
  auth_index: "agent-codex",
  failed: false,
  latency_ms: 1_000,
  ttft_ms: 100,
  output_tps: 42,
  tokens: { output_tokens: 42, total_tokens: 100 },
}

afterEach(cleanup)

describe("RequestEvidenceEvent", () => {
  it("replaces the visible latest-request label with the synchronized signal", () => {
    render(<RequestEvidenceEvent event={event} label="Latest request" syncState="synced" />)

    expect(screen.getByRole("region", { name: "Latest request" })).toBeInTheDocument()
    expect(screen.getByRole("status")).toHaveTextContent("Synced with trend")
    expect(screen.queryByText("Latest request")).not.toBeInTheDocument()
  })

  it("announces the active synchronized refresh", () => {
    render(<RequestEvidenceEvent event={event} label="Latest request" syncState="refreshing" />)

    expect(screen.getByRole("status")).toHaveTextContent("Syncing with trend")
  })

  it("keeps the static label for request drill-down", () => {
    render(<RequestEvidenceEvent event={event} label="Selected request" />)

    expect(screen.getByText("Selected request")).toBeInTheDocument()
    expect(screen.queryByRole("status")).not.toBeInTheDocument()
  })
})
