import { useMemo } from "react"

interface SparklineProps {
  data: (number | null)[]
  color?: string
}

export function Sparkline({ data, color = "#d97757" }: SparklineProps) {
  const validData = useMemo(
    () => data.filter((v): v is number => v !== null),
    [data]
  )

  if (validData.length < 2) {
    return <div className="h-full w-full rounded bg-muted/50" />
  }

  const min = Math.min(...validData)
  const max = Math.max(...validData)
  const range = max - min || 1

  const width = 100
  const height = 32
  const padding = 2

  const points = validData.map((value, index) => {
    const x = padding + (index / (validData.length - 1)) * (width - padding * 2)
    const y = height - padding - ((value - min) / range) * (height - padding * 2)
    return `${x},${y}`
  })

  const areaPath = `M${points[0]} ${points
    .slice(1)
    .map((p) => `L${p}`)
    .join(" ")} L${width - padding},${height} L${padding},${height} Z`

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="h-full w-full" preserveAspectRatio="none">
      <defs>
        <linearGradient id="sparklineGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.3" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={areaPath} fill="url(#sparklineGrad)" />
      <polyline
        points={points.join(" ")}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
