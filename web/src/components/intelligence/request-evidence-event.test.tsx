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
  attempt_facts: {
    generate: true, stream: null, request_service_tier: "priority", response_service_tier: null, output_tps: 42,
    accounting: {
      state: "valid", accounting_version: 2, schema_version: 2, quality: "complete",
      total_tokens: 100,
      input: { total_tokens: 50, uncached_tokens: 44, cache_read_tokens: 5, cache_write_tokens: 1 },
      output: { total_tokens: 50, non_reasoning_tokens: 46, reasoning_tokens: 4 },
      unclassified_tokens: 0,
    },
  },
  tokens: {
    input_tokens: 50,
    output_tokens: 42,
    reasoning_tokens: 4,
    cache_read_tokens: 5,
    cache_write_tokens: 1,
    unclassified_tokens: 0,
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
    expect(screen.getByText("Observed alias label")).toBeInTheDocument()
    expect(screen.queryByText("Requested model")).not.toBeInTheDocument()
    expect(screen.queryByRole("status")).not.toBeInTheDocument()
    expect(screen.getByText("gpt-5-requested")).toBeInTheDocument()
    expect(screen.getByText("gpt-5")).toBeInTheDocument()
    expect(screen.getByText("/v1/responses")).toBeInTheDocument()
    expect(screen.getByText("req-123")).toBeInTheDocument()
    expect(screen.getAllByText("50")).toHaveLength(2)
    expect(screen.getByText("42.0 tok/s")).toBeInTheDocument()
    expect(screen.getByText("4")).toBeInTheDocument()
    expect(screen.getByText("0")).toBeInTheDocument()
    expect(screen.getByText("5")).toBeInTheDocument()
    expect(screen.getByText("1")).toBeInTheDocument()
    expect(screen.getByText("200")).toBeInTheDocument()
    expect(screen.getByText("high")).toBeInTheDocument()
    expect(screen.getByText("priority")).toBeInTheDocument()
    expect(screen.queryByText("Generic cached tokens")).not.toBeInTheDocument()
    expect(screen.getByText("Generate")).toBeInTheDocument()
    expect(screen.getByText("Yes")).toBeInTheDocument()
    expect(screen.getByText("Stream")).toBeInTheDocument()
    expect(screen.getByText("Unknown")).toBeInTheDocument()
  })

  it("shows reported invalid canonical facts, independent tiers and explicit execution absence without estimating TPS", () => {
    render(<RequestEvidenceEvent label="Selected attempt" detail event={{
      ...event,
      output_tps: null,
      attempt_facts: {
        generate: false, stream: null, request_service_tier: "priority", response_service_tier: "default", output_tps: null,
        accounting: {
          state: "invalid", accounting_version: 2, schema_version: 2, quality: "inconsistent",
          total_tokens: 999,
          input: { total_tokens: 100, uncached_tokens: 70, cache_read_tokens: 20, cache_write_tokens: 10 },
          output: { total_tokens: 50, non_reasoning_tokens: 40, reasoning_tokens: 10 },
          unclassified_tokens: 5,
        },
      },
    }} />)
    const value = (label: string) => screen.getByText(label).nextElementSibling
    expect(value("Canonical accounting")).toHaveTextContent("Invalid canonical facts")
    expect(value("Reported quality")).toHaveTextContent("inconsistent")
    expect(value("Canonical total")).toHaveTextContent("999")
    expect(value("Requested service tier")).toHaveTextContent("priority")
    expect(value("Response service tier")).toHaveTextContent("default")
    expect(value("Generate")).toHaveTextContent("No")
    expect(value("Stream")).toHaveTextContent("Unknown")
    expect(value("Output TPS")).toHaveTextContent("-")
  })

  it("does not synthesize canonical facts or response tier for historical evidence", () => {
    render(<RequestEvidenceEvent event={{ ...event, output_tps: null, attempt_facts: undefined }} label="Historical attempt" detail />)
    for (const label of ["Response service tier", "Canonical total", "Accounting version", "Reported quality"]) {
      expect(screen.getByText(label).nextElementSibling).toHaveTextContent("-")
    }
    for (const label of ["Generate", "Stream"]) {
      expect(screen.getByText(label).nextElementSibling).toHaveTextContent("Unknown")
    }
    expect(screen.getByText("Requested service tier").nextElementSibling).toHaveTextContent("priority")
    expect(screen.getByText("Output TPS").nextElementSibling).toHaveTextContent("-")
  })
})
