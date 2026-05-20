import type { ServiceHealth, ServiceHealthBlock } from "@/types/api"
import { useRef, useState, useEffect, useMemo } from "react"

interface HealthGridProps {
  data: ServiceHealth
}

function cellColor(block: ServiceHealthBlock): string {
  if (block.rate < 0) return "bg-muted/20"
  const total = block.success + block.failure
  if (total === 0) return "bg-muted/20"
  if (block.failure === 0) return "bg-emerald-500"
  if (block.rate >= 0.99) return "bg-emerald-400"
  if (block.rate >= 0.95) return "bg-amber-400"
  return "bg-red-400"
}

function formatTimeLabel(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString("en", { hour: "2-digit", minute: "2-digit", hour12: false })
}

function divisors(value: number): number[] {
  const result: number[] = []
  for (let candidate = 1; candidate <= value; candidate++) {
    if (value % candidate === 0) result.push(candidate)
  }
  return result
}

function computeLayout(containerWidth: number, blockCount: number): { columns: number } {
  const labelWidth = 56
  const targetRows = 6
  const targetCellSize = 12
  const minCellSize = 6
  const rowGap = 1
  const availableWidth = Math.max(containerWidth - labelWidth, minCellSize)

  if (blockCount <= 0) {
    return { columns: 1 }
  }

  const maxColumns = Math.max(
    1,
    Math.min(blockCount, Math.floor((availableWidth + rowGap) / (minCellSize + rowGap)))
  )

  const candidates = divisors(blockCount).filter((columns) => columns <= maxColumns)
  if (candidates.length === 0) {
    return { columns: maxColumns }
  }

  const best = candidates.reduce((currentBest, columns) => {
    const rows = Math.ceil(blockCount / columns)
    const cellSize = (availableWidth - (columns - 1) * rowGap) / columns
    const score = Math.abs(rows - targetRows) * 2 + Math.abs(cellSize - targetCellSize)

    const bestRows = Math.ceil(blockCount / currentBest)
    const bestCellSize = (availableWidth - (currentBest - 1) * rowGap) / currentBest
    const bestScore =
      Math.abs(bestRows - targetRows) * 2 + Math.abs(bestCellSize - targetCellSize)

    return score < bestScore ? columns : currentBest
  }, candidates[0])

  return { columns: best }
}

export function HealthGrid({ data }: HealthGridProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [containerWidth, setContainerWidth] = useState(0)

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      setContainerWidth(entries[0].contentRect.width)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const blockCount = data.block_details.length

  const { columns } = useMemo(
    () => computeLayout(containerWidth || 1200, blockCount),
    [containerWidth, blockCount]
  )

  const gridRows: ServiceHealthBlock[][] = useMemo(() => {
    const result: ServiceHealthBlock[][] = []
    for (let start = 0; start < blockCount; start += columns) {
      result.push(data.block_details.slice(start, start + columns))
    }
    return result
  }, [data.block_details, blockCount, columns])

  const labelWidth = 56

  const gridTemplateColumns = useMemo(() => {
    return `${labelWidth}px repeat(${columns}, minmax(6px, 1fr))`
  }, [columns])

  if (blockCount === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
        No health data
      </div>
    )
  }

  return (
    <div ref={containerRef} className="min-w-0 space-y-3">
      {/* Overall stats */}
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 text-xs">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <span className="text-muted-foreground">
            {(data.total_success + data.total_failure).toLocaleString("en")} requests
          </span>
          {data.total_failure > 0 && (
            <span className="text-red-400">{data.total_failure} failed</span>
          )}
        </div>
        <span className="font-medium text-emerald-500">
          {data.success_rate.toFixed(1)}%
        </span>
      </div>

      {/* Grid */}
      <div className="min-w-0 overflow-hidden pb-1">
        <div className="min-w-0 space-y-[2px]">
          {gridRows.map((rowBlocks, rowIdx) => (
            <div
              key={`row-${rowIdx}`}
              className="grid gap-[1px]"
              style={{ gridTemplateColumns }}
            >
              {/* Row label */}
              <div className="flex h-full items-center justify-end truncate pr-2 text-right text-[10px] font-medium tabular-nums text-muted-foreground/60">
                {formatTimeLabel(rowBlocks[0]?.start_time)}
              </div>

              {/* Cells */}
              {Array.from({ length: columns }).map((_, colIdx) => {
                const block = rowBlocks[colIdx]
                if (!block) {
                  return (
                    <div
                      key={`${rowIdx}-${colIdx}`}
                      className="aspect-square rounded-[2px] bg-transparent"
                    />
                  )
                }

                const hasData = block.rate >= 0 && (block.success + block.failure) > 0
                return (
                  <button
                    key={`${rowIdx}-${colIdx}`}
                    className={`group relative aspect-square rounded-[2px] border border-transparent transition-all duration-200 hover:z-10 hover:scale-150 hover:rounded-sm hover:border-foreground/20 hover:shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-500 ${cellColor(block)}`}
                    title={
                      block.rate < 0 || (block.success + block.failure) === 0
                        ? `${formatTimeLabel(block.start_time)} - no data`
                        : `${formatTimeLabel(block.start_time)} - ${(block.rate * 100).toFixed(1)}% success (${block.failure} failures, ${block.success + block.failure} requests)`
                    }
                  >
                    {hasData && block.failure > 0 && (
                      <span className="absolute inset-0 m-auto block h-[2px] w-[2px] rounded-full bg-white/60" />
                    )}
                  </button>
                )
              })}
            </div>
          ))}
        </div>
      </div>

      {/* Legend */}
      <div className="flex flex-wrap items-center gap-3 text-[10px] text-muted-foreground/60">
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-emerald-500" />
          Perfect
        </div>
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-emerald-400" />
          ≥99%
        </div>
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-amber-400" />
          ≥95%
        </div>
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-red-400" />
          &lt;95%
        </div>
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-muted/20 border border-border/30" />
          No data
        </div>
      </div>
    </div>
  )
}
