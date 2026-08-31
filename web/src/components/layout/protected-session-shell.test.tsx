import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { ProtectedSessionShell } from "./protected-session-shell"

const baseProps = {
  session: undefined,
  isLoading: false,
  error: null,
  isRetrying: false,
  onRetry: vi.fn(),
  onUnauthenticated: vi.fn(),
}

describe("ProtectedSessionShell", () => {
  it("fails closed on session errors and retries only the session read", async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()
    const onUnauthenticated = vi.fn()
    render(
      <ProtectedSessionShell {...baseProps} error={new Error("offline")} onRetry={onRetry} onUnauthenticated={onUnauthenticated}>
        <div>Protected dashboard</div>
      </ProtectedSessionShell>,
    )

    expect(screen.getByText("Unable to verify your session")).toBeInTheDocument()
    expect(screen.queryByText("Protected dashboard")).not.toBeInTheDocument()
    expect(onUnauthenticated).not.toHaveBeenCalled()
    await user.click(screen.getByRole("button", { name: "Retry session check" }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it("redirects only an explicit successful unauthenticated result", () => {
    const onUnauthenticated = vi.fn()
    render(
      <ProtectedSessionShell {...baseProps} session={{ authenticated: false }} onUnauthenticated={onUnauthenticated}>
        <div>Protected dashboard</div>
      </ProtectedSessionShell>,
    )

    expect(screen.getByText("Redirecting to sign in...")).toBeInTheDocument()
    expect(onUnauthenticated).toHaveBeenCalledTimes(1)
  })

  it("renders protected content only after authenticated=true", () => {
    render(
      <ProtectedSessionShell {...baseProps} session={{ authenticated: true }}>
        <div>Protected dashboard</div>
      </ProtectedSessionShell>,
    )

    expect(screen.getByText("Protected dashboard")).toBeInTheDocument()
  })
})
