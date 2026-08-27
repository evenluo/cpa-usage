import { beforeEach, describe, expect, it, vi } from "vitest"
import { apiFetch } from "@/lib/api"
import { fetchEvents } from "./useEvents"

vi.mock("@/lib/api", () => ({ apiFetch: vi.fn() }))

const mockedApiFetch = vi.mocked(apiFetch)

beforeEach(() => {
  mockedApiFetch.mockReset()
})

describe("fetchEvents", () => {
  it("accepts a populated page and the normalized empty page", async () => {
    const event = {
      timestamp: "2026-08-27T00:00:00Z",
      model: "gpt-5",
      source: "account",
      failed: false,
      latency_ms: 10,
      ttft_ms: 2,
      output_tps: 100,
      tokens: { output_tokens: 1, total_tokens: 2 },
    }
    mockedApiFetch
      .mockResolvedValueOnce({ events: [event], total_count: 1, page: 1, page_size: 10, total_pages: 1 })
      .mockResolvedValueOnce({ events: [], total_count: 0, page: 1, page_size: 10, total_pages: 1 })

    await expect(fetchEvents("/events?page=1", 1, 10)).resolves.toMatchObject({ events: [event] })
    await expect(fetchEvents("/events?page=1", 1, 10)).resolves.toMatchObject({ events: [], page: 1 })
  })

  it.each([
    ["missing", undefined],
    ["zero", 0],
    ["fractional", 1.5],
    ["negative", -1],
    ["non-finite", Number.POSITIVE_INFINITY],
  ])("rejects %s page metadata", async (_name, invalidPage) => {
    mockedApiFetch.mockResolvedValueOnce({
      events: [],
      total_count: 0,
      page: invalidPage,
      page_size: 10,
      total_pages: 1,
    })

    await expect(fetchEvents("/events", 1, 10)).rejects.toThrow("invalid page")
  })

  it("rejects inconsistent and missing response fields", async () => {
    mockedApiFetch
      .mockResolvedValueOnce({ events: [], total_count: 0, page: 1, page_size: 10, total_pages: 2 })
      .mockResolvedValueOnce({ total_count: 0, page: 1, page_size: 10, total_pages: 1 })

    await expect(fetchEvents("/events", 1, 10)).rejects.toThrow("inconsistent")
    await expect(fetchEvents("/events", 1, 10)).rejects.toThrow("invalid items")
  })

  it("rejects a normalized empty response when it does not match the requested page", async () => {
    mockedApiFetch.mockResolvedValueOnce({
      events: [],
      total_count: 0,
      page: 1,
      page_size: 10,
      total_pages: 1,
    })

    await expect(fetchEvents("/events?page=7", 7, 10)).rejects.toThrow("page 1 while page 7 was requested")
  })

  it("rejects a short populated event page instead of publishing partial evidence", async () => {
    mockedApiFetch.mockResolvedValueOnce({
      events: [{
        timestamp: "2026-08-27T00:00:00Z",
        model: "gpt-5",
        source: "account",
        failed: false,
        latency_ms: 10,
        ttft_ms: 2,
        output_tps: 100,
        tokens: { output_tokens: 1, total_tokens: 2 },
      }],
      total_count: 2,
      page: 1,
      page_size: 10,
      total_pages: 1,
    })

    await expect(fetchEvents("/events", 1, 10)).rejects.toThrow("incomplete page item count")
  })
})
