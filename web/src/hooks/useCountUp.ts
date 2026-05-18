import { useEffect, useRef, useState } from "react"

function easeOutQuart(t: number): number {
  return 1 - Math.pow(1 - t, 4)
}

export function useCountUp(
  target: number,
  options: { duration?: number; decimals?: number; enabled?: boolean } = {}
) {
  const { duration = 800, decimals = 0, enabled = true } = options
  const [value, setValue] = useState(target)
  const startRef = useRef<number | null>(null)
  const fromRef = useRef(0)
  const targetRef = useRef(target)
  const valueRef = useRef(target)

  useEffect(() => {
    if (!enabled) {
      return
    }

    fromRef.current = valueRef.current
    targetRef.current = target
    startRef.current = null

    let rafId: number

    const tick = (timestamp: number) => {
      if (startRef.current === null) startRef.current = timestamp
      const elapsed = timestamp - startRef.current
      const progress = Math.min(elapsed / duration, 1)
      const eased = easeOutQuart(progress)
      const current = fromRef.current + (targetRef.current - fromRef.current) * eased
      valueRef.current = current
      setValue(current)

      if (progress < 1) {
        rafId = requestAnimationFrame(tick)
      }
    }

    rafId = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafId)
  }, [target, duration, enabled])

  const factor = Math.pow(10, decimals)
  const displayValue = enabled ? value : target
  return Math.round(displayValue * factor) / factor
}
