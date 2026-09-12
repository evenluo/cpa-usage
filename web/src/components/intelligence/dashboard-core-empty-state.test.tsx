import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import { DashboardCoreEmptyState } from "./dashboard-core-empty-state"

afterEach(cleanup)

describe("DashboardCoreEmptyState", () => {
  it("renders a successful empty result without a retry", () => {
    render(<DashboardCoreEmptyState onRetry={vi.fn()} />)

    expect(screen.getByText("No usage")).toBeInTheDocument()
    expect(screen.queryByRole("button")).not.toBeInTheDocument()
  })

  it("keeps an empty cached result while exposing refresh failure and scoped retry", async () => {
    const user = userEvent.setup()
    const retryCore = vi.fn()
    render(<DashboardCoreEmptyState refreshError={new Error("refresh failed")} onRetry={retryCore} />)

    expect(screen.getByText("Refresh failed. Last result was empty.")).toBeInTheDocument()
    expect(screen.queryByText("No usage")).not.toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Retry" }))
    expect(retryCore).toHaveBeenCalledTimes(1)
  })
})
