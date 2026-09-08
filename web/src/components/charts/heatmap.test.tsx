import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import type { HeatmapCell, HeatmapData } from "@/types/api"
import { Heatmap } from "./heatmap"

class ResizeObserverMock {
  constructor(private readonly callback: ResizeObserverCallback) {}
  observe(target: Element) {
    this.callback([
      { target, contentRect: { width: 400 } } as ResizeObserverEntry,
    ], this as unknown as ResizeObserver)
  }
  unobserve() {}
  disconnect() {}
}

beforeAll(() => {
  vi.stubGlobal("ResizeObserver", ResizeObserverMock)
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function cell(hour: number, overrides: Partial<HeatmapCell> = {}): HeatmapCell {
  return {
    hour,
    in_range: true,
    bucket_start: `2026-05-11T${hour.toString().padStart(2, "0")}:00:00Z`,
    bucket_end: `2026-05-11T${(hour + 1).toString().padStart(2, "0")}:00:00Z`,
    total_tokens: 100,
    total_cost: 0.5,
    request_count: 2,
    failure_count: 0,
    cost_available: true,
    cost_status: "available",
    canonical_valid_attempts: 2,
    ...overrides,
  }
}

function data(cells: HeatmapCell[], overrides: Partial<HeatmapData> = {}): HeatmapData {
  return {
    measure: "tokens",
    max_tokens: 200,
    max_cost: 1,
    max_requests: 4,
    max_failures: 1,
    rows: [{ date: "2026-05-11", label: "05/11 Mon", cells }],
    ...overrides,
  }
}

describe("Heatmap", () => {
  it("defaults to Tokens and switches the local metric scale without replacing cells", async () => {
    const user = userEvent.setup()
    render(<Heatmap data={data([cell(0, { failure_count: 1 })])} />)

    expect(screen.getByRole("button", { name: "Tokens" })).toHaveAttribute("aria-pressed", "true")
    expect(screen.getByLabelText("Tokens legend")).toHaveTextContent("Tokens: 0–200")
    expect(screen.getByLabelText("Tokens legend")).toHaveTextContent("Observed zero")
    expect(screen.getByLabelText("Tokens legend")).toHaveTextContent("Outside range")
    expect(screen.getByRole("note")).toHaveTextContent("Swipe horizontally to view all hours")
    expect(screen.getByRole("button", { name: /05\/11 Mon 00:00/ }).parentElement?.style.gridTemplateColumns).toContain("68px 24px")
    expect(screen.getByText("05/11").parentElement).toHaveClass("sticky", "left-0")
    const activityCell = screen.getByRole("button", { name: /05\/11 Mon 00:00/ })

    await user.click(screen.getByRole("button", { name: "Attempts" }))
    expect(screen.getByRole("button", { name: "Attempts" })).toHaveAttribute("aria-pressed", "true")
    expect(screen.getByLabelText("Attempts legend")).toHaveTextContent("Attempts: 0–4")
    expect(screen.getByRole("button", { name: /05\/11 Mon 00:00/ })).toBe(activityCell)

    await user.click(screen.getByRole("button", { name: "Failures" }))
    expect(screen.getByLabelText("Failures legend")).toHaveTextContent("Failures: 0–1")
  })

  it("exposes no activity, unavailable, partial, true zero, and out-of-range as distinct states", () => {
    const { container } = render(
      <Heatmap
        data={data([
          cell(0, { request_count: 0, canonical_valid_attempts: 0, total_tokens: 0 }),
          cell(1, { canonical_valid_attempts: 0 }),
          cell(2, { canonical_valid_attempts: 1 }),
          cell(3, { total_tokens: 0 }),
          cell(4, { in_range: false }),
        ])}
      />,
    )

    expect(screen.getByRole("button", { name: /00:00 · No activity/ })).toHaveAttribute("data-state", "no-activity")
    expect(screen.getByRole("button", { name: /01:00.*Tokens unavailable/ })).toHaveAttribute("data-state", "token-unavailable")
    expect(screen.getByRole("button", { name: /02:00.*Token coverage 1\/2/ })).toHaveAttribute("data-state", "token-partial")
    expect(screen.getByRole("button", { name: /03:00.*Tokens 0/ })).toHaveAttribute("data-state", "observed")
    expect(container.querySelector('[data-state="out-of-range"]')).toHaveAttribute("aria-label", expect.stringContaining("Outside selected range"))
  })

  it("shows complete details on focus and click, closes with Escape, and keeps the tooltip in the viewport", () => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
      if (this.getAttribute("role") === "tooltip") {
        return { x: 0, y: 0, top: 0, left: 0, right: 320, bottom: 60, width: 320, height: 60, toJSON: () => ({}) }
      }
      return { x: 1000, y: 740, top: 740, left: 1000, right: 1012, bottom: 752, width: 12, height: 12, toJSON: () => ({}) }
    })
    render(<Heatmap data={data([cell(0, { canonical_valid_attempts: 1, cost_available: false, cost_status: "partial" })])} />)
    const activityCell = screen.getByRole("button", { name: /05\/11 Mon 00:00/ })

    fireEvent.focus(activityCell)
    const tooltip = screen.getByRole("tooltip")
    expect(tooltip).toHaveTextContent("Attempts 2")
    expect(tooltip).toHaveTextContent("Failures 0")
    expect(tooltip).toHaveTextContent("Tokens 100")
    expect(tooltip).toHaveTextContent("Token coverage 1/2")
    expect(tooltip).toHaveTextContent("Cost partial")
    expect(Number.parseFloat(tooltip.style.left)).toBeLessThanOrEqual(window.innerWidth - 328)
    expect(Number.parseFloat(tooltip.style.top)).toBeLessThanOrEqual(window.innerHeight - 68)

    fireEvent.keyDown(activityCell, { key: "Escape" })
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument()
    fireEvent.click(activityCell)
    expect(screen.getByRole("tooltip")).toBeInTheDocument()
  })

  it("retains focused details when adjacent loading moves the pointer away", async () => {
    const user = userEvent.setup()
    render(<Heatmap data={data([cell(0)])} />)
    const activityCell = screen.getByRole("button", { name: /05\/11 Mon 00:00/ })
    await user.click(activityCell)
    expect(activityCell).toHaveFocus()
    fireEvent.mouseLeave(activityCell)
    expect(screen.getByRole("tooltip")).toHaveTextContent("Attempts 2")
    await user.tab()
    expect(activityCell).not.toHaveFocus()
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument()
    fireEvent.mouseEnter(activityCell)
    fireEvent.mouseLeave(activityCell)
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument()
  })

  it("keeps an active all-zero failure grid and explains the zero state", async () => {
    const user = userEvent.setup()
    render(<Heatmap data={data([cell(0)], { max_failures: 0 })} />)

    await user.click(screen.getByRole("button", { name: "Failures" }))

    expect(screen.getByRole("status")).toHaveTextContent("No failures in this period")
    expect(screen.getByRole("button", { name: /05\/11 Mon 00:00/ })).toHaveAttribute("data-state", "observed")
  })
})
