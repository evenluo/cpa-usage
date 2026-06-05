import { type RefObject, useLayoutEffect, useRef } from "react"

interface UseFlipReorderOptions {
  enabled: boolean
  duration?: number
  easing?: string
}

interface UseFlipReorderResult {
  containerRef: RefObject<HTMLDivElement | null>
  registerItem: (key: string) => (element: HTMLDivElement | null) => void
}

export function useFlipReorder(
  itemKeys: string[],
  options: UseFlipReorderOptions,
): UseFlipReorderResult {
  const { enabled, duration = 300, easing = "ease-out" } = options

  const containerRef = useRef<HTMLDivElement | null>(null)
  const elementMapRef = useRef(new Map<string, HTMLDivElement>())
  const positionsRef = useRef(new Map<string, { x: number; y: number }>())
  const prevKeysRef = useRef<string[]>([])
  const isFirstRunRef = useRef(true)
  const animationsRef = useRef<Animation[]>([])
  const registerItemCacheRef = useRef(new Map<string, (element: HTMLDivElement | null) => void>())

  const registerItem = (key: string): ((element: HTMLDivElement | null) => void) => {
    let callback = registerItemCacheRef.current.get(key)
    if (!callback) {
      callback = (element: HTMLDivElement | null) => {
        if (element) {
          elementMapRef.current.set(key, element)
        } else {
          elementMapRef.current.delete(key)
        }
      }
      registerItemCacheRef.current.set(key, callback)
    }
    return callback
  }

  useLayoutEffect(() => {
    const container = containerRef.current
    if (!container || !enabled) {
      if (!enabled) {
        positionsRef.current = new Map()
        prevKeysRef.current = []
        isFirstRunRef.current = true
      }
      return
    }

    const oldPositions = positionsRef.current

    // Measure new positions relative to container
    const containerRect = container.getBoundingClientRect()
    const newPositions = new Map<string, { x: number; y: number }>()

    for (const [key, element] of elementMapRef.current) {
      const rect = element.getBoundingClientRect()
      newPositions.set(key, {
        x: rect.left - containerRect.left,
        y: rect.top - containerRect.top,
      })
    }

    // Skip animation on first render — just record positions
    if (isFirstRunRef.current) {
      isFirstRunRef.current = false
      positionsRef.current = newPositions
      prevKeysRef.current = [...itemKeys]
      return
    }

    // Respect prefers-reduced-motion
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      positionsRef.current = newPositions
      prevKeysRef.current = [...itemKeys]
      return
    }

    // Cancel any in-flight animations from the previous cycle
    for (const anim of animationsRef.current) {
      anim.cancel()
    }
    animationsRef.current = []

    // FLIP: invert and play
    const prevKeysSet = new Set(prevKeysRef.current)
    const newKeysSet = new Set(itemKeys)

    for (const [key, element] of elementMapRef.current) {
      // Only animate elements that exist in both the previous and current frame
      if (!prevKeysSet.has(key) || !newKeysSet.has(key)) continue

      const oldPos = oldPositions.get(key)
      const newPos = newPositions.get(key)
      if (!oldPos || !newPos) continue

      const dx = oldPos.x - newPos.x
      const dy = oldPos.y - newPos.y

      // Skip sub-pixel differences
      if (Math.abs(dx) < 1 && Math.abs(dy) < 1) continue

      const animation = element.animate(
        [
          { transform: `translate(${dx}px, ${dy}px)` },
          { transform: "translate(0, 0)" },
        ],
        { duration, easing, fill: "none" },
      )
      animationsRef.current.push(animation)
    }

    // Store current positions for the next cycle
    positionsRef.current = newPositions
    prevKeysRef.current = [...itemKeys]
  }) // No dependency array — runs after every render to capture positions

  // Cleanup on unmount
  useLayoutEffect(() => {
    return () => {
      for (const anim of animationsRef.current) {
        anim.cancel()
      }
      animationsRef.current = []
    }
  }, [])

  return { containerRef, registerItem }
}
