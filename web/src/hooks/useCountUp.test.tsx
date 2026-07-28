import { act, renderHook } from "@testing-library/react"
import { afterEach, expect, it, vi } from "vitest"
import { useCountUp } from "./useCountUp"

afterEach(() => {
  vi.restoreAllMocks()
})

it("updates immediately when reduced motion is requested", () => {
  window.matchMedia = vi.fn().mockReturnValue({
    matches: true,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }) as unknown as typeof window.matchMedia
  const { result, rerender } = renderHook(
    ({ target }) => useCountUp(target),
    { initialProps: { target: 10 } },
  )

  act(() => rerender({ target: 25 }))

  expect(result.current).toBe(25)
})
