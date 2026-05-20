import type { HeatmapData, HeatmapCell } from "@/types/api"
import { formatCompact, formatCost } from "@/lib/format"
import { useRef, useState, useEffect, useMemo } from "react"
import type { FocusEvent, MouseEvent } from "react"

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

interface HeatmapTooltip {
  label: string
  x: number
  y: number
}

interface HeatmapRowLabel {
  dateLabel: string
  weekdayLabel: string
}

const heatmapLabelWidth = 68

function cellTooltipLabel(fc: FlatCell): string {
  if (!fc.cell) return ""
  return `${fc.dateLabel} ${fc.hour}:00 · ${formatCompact(fc.cell.total_tokens, 1)}t · ${fc.cell.request_count}r · ${cellCostLabel(fc.cell)}`
}

function fallbackDateLabel(date: string): string {
  const match = date.match(/^\d{4}-(\d{2})-(\d{2})$/)
  return match ? `${match[1]}/${match[2]}` : date
}

function splitRowLabel(label: string, date: string): HeatmapRowLabel {
  const dateLabel = label.match(/\d{1,2}\/\d{1,2}/)?.[0] ?? fallbackDateLabel(date)
  const weekdayLabel = label.match(/[A-Za-z]{3,}/)?.[0] ?? ""
  return { dateLabel, weekdayLabel }
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
  const hoursPerDay = 24
  const gap = 1
  const daySep = 3
  const minCellSize = 8

  for (let daysPerRow = 3; daysPerRow >= 1; daysPerRow--) {
    const cols = hoursPerDay * daysPerRow
    const totalGap = (cols - 1) * gap + (daysPerRow - 1) * daySep
    const cellSize = Math.floor((containerWidth - heatmapLabelWidth - totalGap) / cols)
    if (cellSize >= minCellSize) {
      return { daysPerRow, cellSize }
    }
  }

  return { daysPerRow: 1, cellSize: 12 }
}

export function Heatmap({ data }: HeatmapProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [containerWidth, setContainerWidth] = useState(0)
  const [tooltip, setTooltip] = useState<HeatmapTooltip | null>(null)

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      setContainerWidth(entries[0].contentRect.width)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const { cells: flatCells, maxTokens } = useMemo(() => flattenCells(data), [data])

  const { daysPerRow, cellSize } = useMemo(
    () => computeLayout(containerWidth || 1200),
    [containerWidth]
  )

  if (data.rows.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
        No heatmap data in this range
      </div>
    )
  }

  const colsPerRow = 24 * daysPerRow

  const showPointerTooltip = (event: MouseEvent<HTMLElement>, fc: FlatCell) => {
    const label = cellTooltipLabel(fc)
    if (!label) return
    setTooltip({ label, x: event.clientX + 12, y: event.clientY + 12 })
  }

  const showFocusTooltip = (event: FocusEvent<HTMLElement>, fc: FlatCell) => {
    const label = cellTooltipLabel(fc)
    if (!label) return
    const rect = event.currentTarget.getBoundingClientRect()
    setTooltip({ label, x: rect.left + rect.width + 8, y: rect.top + rect.height + 8 })
  }

  const hideTooltip = () => setTooltip(null)

  // Split into rows
  const rows: Array<{ label: HeatmapRowLabel; cells: FlatCell[] }> = []
  for (let i = 0; i < flatCells.length; i += colsPerRow) {
    const rowCells = flatCells.slice(i, i + colsPerRow)
    const firstCell = rowCells[0]
    rows.push({
      label: firstCell ? splitRowLabel(firstCell.dateLabel, firstCell.date) : { dateLabel: "", weekdayLabel: "" },
      cells: rowCells,
    })
  }

  // Build grid columns: label + [24 cells + separator] × (daysPerRow - 1) + 24 cells
  const daySep = 3
  const gridCols: string[] = [`${heatmapLabelWidth}px`]
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
              <div className="grid h-full grid-cols-[5ch_3ch] items-center gap-1 pr-2 text-[10px] font-medium text-muted-foreground/60">
                <span className="text-right tabular-nums">{row.label.dateLabel}</span>
                <span className="text-left">{row.label.weekdayLabel}</span>
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
                        onMouseEnter={(event) => showPointerTooltip(event, fc)}
                        onMouseMove={(event) => showPointerTooltip(event, fc)}
                        onMouseLeave={hideTooltip}
                        onFocus={(event) => showFocusTooltip(event, fc)}
                        onBlur={hideTooltip}
                      >
                        {intensity > 0.6 && (
                          <span className="absolute inset-0 m-auto block h-[2px] w-[2px] rounded-full bg-terracotta-500/60" />
                        )}
                      </button>
                    )
                  })()
                  return (
                    <div key={`day-${rowIdx}-${ci}`} className="contents">
                      {sep}
                      {cell}
                    </div>
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
                    onMouseEnter={(event) => showPointerTooltip(event, fc)}
                    onMouseMove={(event) => showPointerTooltip(event, fc)}
                    onMouseLeave={hideTooltip}
                    onFocus={(event) => showFocusTooltip(event, fc)}
                    onBlur={hideTooltip}
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
      {tooltip && (
        <div
          className="pointer-events-none fixed z-50 rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-lg"
          style={{ left: tooltip.x, top: tooltip.y }}
        >
          {tooltip.label}
        </div>
      )}
    </div>
  )
}
