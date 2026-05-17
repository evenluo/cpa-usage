import { useEffect, useRef, useState } from "react"

function easeOutQuart(t: number): number {
  return 1 - Math.pow(1 - t, 4)
}

export function useCountUp(
  target: number,
  options: { duration?: number; decimals?: number; enabled?: boolean } = {}
) {
  const { duration = 800, decimals = 0, enabled = true } = options
  const [value, setValue] = useState(0)
  const startRef = useRef<number | null>(null)
  const fromRef = useRef(0)
  const targetRef = useRef(target)

  useEffect(() => {
    if (!enabled) {
      setValue(target)
      return
    }

    fromRef.current = value
    targetRef.current = target
    startRef.current = null

    let rafId: number

    const tick = (timestamp: number) => {
      if (startRef.current === null) startRef.current = timestamp
      const elapsed = timestamp - startRef.current
      const progress = Math.min(elapsed / duration, 1)
      const eased = easeOutQuart(progress)
      const current = fromRef.current + (targetRef.current - fromRef.current) * eased
      setValue(current)

      if (progress < 1) {
        rafId = requestAnimationFrame(tick)
      }
    }

    rafId = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafId)
  }, [target, duration, enabled])

  const factor = Math.pow(10, decimals)
  return Math.round(value * factor) / factor
}
