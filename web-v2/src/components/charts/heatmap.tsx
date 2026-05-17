import type { HeatmapData, HeatmapCell } from "@/types/api"
import { formatCompact, formatCost } from "@/lib/format"
import { useRef, useState, useEffect, useMemo } from "react"

interface HeatmapProps {
  data: HeatmapData
}

function cellCostLabel(cell: Pick<HeatmapCell, "cost_available" | "cost_status" | "total_cost">): string {
  if (!cell.cost_available) {
    return cell.cost_status === "partial" ? "partial" : "n/a"
  }
  return formatCost(cell.total_cost)
}

interface FlatCell {
  date: string
  dateLabel: string
  hour: number
  cell: HeatmapCell | null
}

function flattenCells(data: HeatmapData): {
  cells: FlatCell[]
  maxTokens: number
} {
  const maxTokens = Math.max(data.max_tokens, 1)
  const flat: FlatCell[] = []

  for (const row of data.rows) {
    const cellMap = new Map(row.cells.map((c) => [c.hour, c]))
    for (let h = 0; h < 24; h++) {
      flat.push({
        date: row.date,
        dateLabel: row.label,
        hour: h,
        cell: cellMap.get(h) ?? null,
      })
    }
  }

  return { cells: flat, maxTokens }
}

function computeLayout(containerWidth: number): { daysPerRow: number; cellSize: number } {
  const labelWidth = 56
  const hoursPerDay = 24
  const gap = 1
  const daySep = 3
  const minCellSize = 8

  for (let daysPerRow = 3; daysPerRow >= 1; daysPerRow--) {
    const cols = hoursPerDay * daysPerRow
    const totalGap = (cols - 1) * gap + (daysPerRow - 1) * daySep
    const cellSize = Math.floor((containerWidth - labelWidth - totalGap) / cols)
    if (cellSize >= minCellSize) {
      return { daysPerRow, cellSize }
    }
  }

  return { daysPerRow: 1, cellSize: 12 }
}

export function Heatmap({ data }: HeatmapProps) {
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

  if (data.rows.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
        No heatmap data in this range
      </div>
    )
  }

  const { cells: flatCells, maxTokens } = useMemo(() => flattenCells(data), [data])

  const { daysPerRow, cellSize } = useMemo(
    () => computeLayout(containerWidth || 1200),
    [containerWidth]
  )

  const colsPerRow = 24 * daysPerRow

  // Split into rows
  const rows: Array<{ startLabel: string; cells: FlatCell[] }> = []
  for (let i = 0; i < flatCells.length; i += colsPerRow) {
    const rowCells = flatCells.slice(i, i + colsPerRow)
    rows.push({
      startLabel: rowCells[0]?.dateLabel ?? "",
      cells: rowCells,
    })
  }

  // Build grid columns: label + [24 cells + separator] × (daysPerRow - 1) + 24 cells
  const labelWidth = 56
  const daySep = 3
  const gridCols: string[] = [`${labelWidth}px`]
  for (let d = 0; d < daysPerRow; d++) {
    if (d > 0) gridCols.push(`${daySep}px`)
    for (let h = 0; h < 24; h++) {
      gridCols.push(`${cellSize}px`)
    }
  }
  const gridTemplateColumns = gridCols.join(' ')

  return (
    <div ref={containerRef} className="overflow-x-auto pb-1">
      <div className="space-y-2">
        {rows.map((row, rowIdx) => {
          const cells = row.cells

          return (
            <div
              key={`row-${rowIdx}`}
              className="grid gap-[1px]"
              style={{ gridTemplateColumns }}
            >
              {/* Row start date label */}
              <div className="flex h-full items-center truncate pr-2 text-[10px] font-medium text-muted-foreground/60">
                {row.startLabel}
              </div>

              {/* Cells + separators */}
              {cells.map((fc, ci) => {
                const isDayStart = ci > 0 && ci % 24 === 0

                if (isDayStart) {
                  // Weak separator between days
                  const sep = (
                    <div
                      key={`sep-${rowIdx}-${ci}`}
                      className="rounded-full bg-border/20"
                      style={{ width: `${daySep - 1}px`, marginLeft: '1px' }}
                    />
                  )
                  const cell = (() => {
                    if (!fc.cell) {
                      return (
                        <div
                          key={`${rowIdx}-${ci}`}
                          className="aspect-square rounded-[2px] bg-muted/20"
                        />
                      )
                    }
                    const intensity = fc.cell.in_range
                      ? Math.min(fc.cell.total_tokens / maxTokens, 1)
                      : 0
                    const alpha = fc.cell.in_range ? 0.1 + intensity * 0.85 : 0.04
                    return (
                      <button
                        key={`${rowIdx}-${ci}`}
                        className="group relative aspect-square rounded-[2px] border border-transparent transition-all duration-200 hover:z-10 hover:scale-150 hover:rounded-sm hover:border-terracotta-300 hover:shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
                        style={{
                          backgroundColor: fc.cell.in_range
                            ? `rgba(217, 119, 87, ${alpha})`
                            : "rgba(113, 113, 122, 0.04)",
                        }}
                        title={`${fc.dateLabel} ${fc.hour}:00 · ${formatCompact(fc.cell.total_tokens, 1)}t · ${fc.cell.request_count}r · ${cellCostLabel(fc.cell)}`}
                      >
                        {intensity > 0.6 && (
                          <span className="absolute inset-0 m-auto block h-[2px] w-[2px] rounded-full bg-terracotta-500/60" />
                        )}
                      </button>
                    )
                  })()
                  return (
                    <>
                      {sep}
                      {cell}
                    </>
                  )
                }

                if (!fc.cell) {
                  return (
                    <div
                      key={`${rowIdx}-${ci}`}
                      className="aspect-square rounded-[2px] bg-muted/20"
                    />
                  )
                }

                const intensity = fc.cell.in_range
                  ? Math.min(fc.cell.total_tokens / maxTokens, 1)
                  : 0
                const alpha = fc.cell.in_range ? 0.1 + intensity * 0.85 : 0.04

                return (
                  <button
                    key={`${rowIdx}-${ci}`}
                    className="group relative aspect-square rounded-[2px] border border-transparent transition-all duration-200 hover:z-10 hover:scale-150 hover:rounded-sm hover:border-terracotta-300 hover:shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
                    style={{
                      backgroundColor: fc.cell.in_range
                        ? `rgba(217, 119, 87, ${alpha})`
                        : "rgba(113, 113, 122, 0.04)",
                    }}
                    title={`${fc.dateLabel} ${fc.hour}:00 · ${formatCompact(fc.cell.total_tokens, 1)}t · ${fc.cell.request_count}r · ${cellCostLabel(fc.cell)}`}
                  >
                    {intensity > 0.6 && (
                      <span className="absolute inset-0 m-auto block h-[2px] w-[2px] rounded-full bg-terracotta-500/60" />
                    )}
                  </button>
                )
              })}
            </div>
          )
        })}
      </div>
    </div>
  )
}
