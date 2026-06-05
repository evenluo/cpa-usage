import { act, renderHook } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { useFlipReorder } from "./useFlipReorder"

// jsdom does not implement getBoundingClientRect or Web Animations API
const mockAnimate = vi.fn(() => ({
  cancel: vi.fn(),
  finished: Promise.resolve(),
}))

beforeEach(() => {
  Element.prototype.animate = mockAnimate as unknown as typeof Element.prototype.animate
  window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia
})

afterEach(() => {
  vi.restoreAllMocks()
})

function createMockElement(key: string, x: number, y: number): HTMLDivElement {
  const el = document.createElement("div")
  el.dataset.testKey = key
  el.getBoundingClientRect = vi.fn(() => ({
    left: x,
    top: y,
    width: 300,
    height: 190,
    right: x + 300,
    bottom: y + 190,
    x,
    y,
    toJSON: () => ({}),
  }))
  return el
}

function createContainerElement(): HTMLDivElement {
  const container = document.createElement("div")
  container.getBoundingClientRect = vi.fn(() => ({
    left: 0,
    top: 0,
    width: 900,
    height: 560,
    right: 900,
    bottom: 560,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  }))
  return container
}

describe("useFlipReorder", () => {
  it("does not animate on the first render", () => {
    const container = createContainerElement()
    const elA = createMockElement("a", 0, 0)
    const elB = createMockElement("b", 0, 200)

    const { result } = renderHook(() =>
      useFlipReorder(["a", "b"], { enabled: true }),
    )

    // Attach container and items via refs
    act(() => {
      (result.current.containerRef as { current: HTMLDivElement | null }).current = container
      result.current.registerItem("a")(elA)
      result.current.registerItem("b")(elB)
    })

    // Re-render to trigger the layout effect with refs attached
    const { result: result2 } = renderHook(() =>
      useFlipReorder(["a", "b"], { enabled: true }),
    )
    act(() => {
      (result2.current.containerRef as { current: HTMLDivElement | null }).current = container
      result2.current.registerItem("a")(elA)
      result2.current.registerItem("b")(elB)
    })

    expect(mockAnimate).not.toHaveBeenCalled()
  })

  it("does not animate when enabled is false", () => {
    const container = createContainerElement()
    const elA = createMockElement("a", 0, 0)

    renderHook(() => useFlipReorder(["a"], { enabled: false }))

    expect(mockAnimate).not.toHaveBeenCalled()
    // registerItem with enabled=false should not produce animations
    const { result } = renderHook(() => useFlipReorder(["a"], { enabled: false }))
    act(() => {
      (result.current.containerRef as { current: HTMLDivElement | null }).current = container
      result.current.registerItem("a")(elA)
    })

    expect(mockAnimate).not.toHaveBeenCalled()
  })

  it("returns stable registerItem callbacks for the same key", () => {
    const { result, rerender } = renderHook(() =>
      useFlipReorder(["a"], { enabled: true }),
    )

    const first = result.current.registerItem("a")
    rerender()
    const second = result.current.registerItem("a")

    expect(first).toBe(second)
  })

  it("cleans up element references when registerItem is called with null", () => {
    const { result } = renderHook(() =>
      useFlipReorder(["a"], { enabled: true }),
    )

    const el = createMockElement("a", 0, 0)
    act(() => {
      result.current.registerItem("a")(el)
      result.current.registerItem("a")(null) // unregister
    })

    // Should not throw or produce animations
    expect(mockAnimate).not.toHaveBeenCalled()
  })

  it("does not animate when prefers-reduced-motion is set", () => {
    window.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof window.matchMedia

    const container = createContainerElement()
    const elA = createMockElement("a", 0, 0)

    const { result } = renderHook(() =>
      useFlipReorder(["a"], { enabled: true }),
    )

    act(() => {
      (result.current.containerRef as { current: HTMLDivElement | null }).current = container
      result.current.registerItem("a")(elA)
    })

    expect(mockAnimate).not.toHaveBeenCalled()
  })
})
