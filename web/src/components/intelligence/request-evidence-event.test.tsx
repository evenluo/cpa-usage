import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it } from "vitest"
import type { UsageEvent } from "@/types/api"
import { RequestEvidenceEvent } from "./request-evidence-event"

const event: UsageEvent = {
  timestamp: "2026-07-14T12:00:00+08:00",
  model: "gpt-5",
  model_alias: "gpt-5-requested",
  endpoint: "/v1/responses",
  request_id: "req-123",
  status_code: 200,
  executor_type: "openai",
  reasoning_effort: "high",
  service_tier: "priority",
  source: "Codex",
  auth_index: "agent-codex",
  failed: false,
  latency_ms: 1_000,
  ttft_ms: 100,
  output_tps: 42,
  tokens: {
    input_tokens: 50,
    output_tokens: 42,
    reasoning_tokens: 4,
    cached_tokens: 0,
    cache_read_tokens: 5,
    cache_creation_tokens: 1,
    total_tokens: 100,
  },
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
    render(<RequestEvidenceEvent event={event} label="Selected upstream attempt" detail />)

    expect(screen.getByText("Selected upstream attempt")).toBeInTheDocument()
    expect(screen.queryByRole("status")).not.toBeInTheDocument()
    expect(screen.getByText("gpt-5-requested")).toBeInTheDocument()
    expect(screen.getByText("gpt-5")).toBeInTheDocument()
    expect(screen.getByText("/v1/responses")).toBeInTheDocument()
    expect(screen.getByText("req-123")).toBeInTheDocument()
    expect(screen.getByText("50")).toBeInTheDocument()
    expect(screen.getByText("42")).toBeInTheDocument()
    expect(screen.getByText("4")).toBeInTheDocument()
    expect(screen.getByText("0")).toBeInTheDocument()
    expect(screen.getByText("5")).toBeInTheDocument()
    expect(screen.getByText("1")).toBeInTheDocument()
    expect(screen.getByText("200")).toBeInTheDocument()
    expect(screen.getByText("high")).toBeInTheDocument()
    expect(screen.getByText("priority")).toBeInTheDocument()
    expect(screen.getByText("Generic cached tokens")).toBeInTheDocument()
  })
})
