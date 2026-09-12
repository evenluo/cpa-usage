import { Fragment, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import type { FocusEvent, MouseEvent } from "react"
import type { HeatmapCell, HeatmapData } from "@/types/api"
import { formatCompact, formatCost } from "@/lib/format"
import {
  computeLayout,
  flattenCells,
  heatmapCellDatum,
  heatmapLabelWidth,
  metricMaximum,
  splitRowLabel,
  type FlatCell,
  type HeatmapMetric,
  type HeatmapRowLabel,
} from "@/features/usage-intelligence/heatmap-model"

interface HeatmapProps { data: HeatmapData }

interface HeatmapTooltip { label: string; x: number; y: number }

const metricOptions: Array<{ value: HeatmapMetric; label: string }> = [
  { value: "tokens", label: "Tokens" },
  { value: "attempts", label: "Attempts" },
  { value: "failures", label: "Failures" },
]

const exactNumber = new Intl.NumberFormat("en-US")

function cellCostLabel(cell: Pick<HeatmapCell, "cost_available" | "cost_status" | "total_cost">): string {
  if (!cell.cost_available) return cell.cost_status === "partial" ? "Cost incomplete" : "Cost unavailable"
  return `Cost ${formatCost(cell.total_cost)}`
}

function tokenLabel(fc: FlatCell): string {
  if (!fc.cell || fc.cell.canonical_valid_attempts === 0) return "Tokens unavailable"
  return `Tokens ${exactNumber.format(fc.cell.total_tokens)}`
}

function cellTooltipLabel(fc: FlatCell): string {
  if (!fc.cell || !fc.cell.in_range) return ""
  const hour = `${fc.hour.toString().padStart(2, "0")}:00`
  return [
    `${fc.dateLabel} ${hour}`,
    `${exactNumber.format(fc.cell.request_count)} attempts`,
    tokenLabel(fc),
    cellCostLabel(fc.cell),
  ].join(" · ")
}

function metricColor(metric: HeatmapMetric, alpha: number): string {
  return metric === "failures" ? `rgba(220, 78, 70, ${alpha})` : `rgba(217, 119, 87, ${alpha})`
}

export function Heatmap({ data }: HeatmapProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)
  const [containerWidth, setContainerWidth] = useState(0)
  const [metric, setMetric] = useState<HeatmapMetric>("tokens")
  const [tooltip, setTooltip] = useState<HeatmapTooltip | null>(null)

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => setContainerWidth(entries[0].contentRect.width))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  useLayoutEffect(() => {
    const el = tooltipRef.current
    if (!el || !tooltip) return
    const rect = el.getBoundingClientRect()
    const nextX = Math.max(8, Math.min(tooltip.x, window.innerWidth - rect.width - 8))
    const nextY = Math.max(8, Math.min(tooltip.y, window.innerHeight - rect.height - 8))
    if (nextX !== tooltip.x || nextY !== tooltip.y) {
      setTooltip({ ...tooltip, x: nextX, y: nextY })
    }
  }, [tooltip])

  const flatCells = useMemo(() => flattenCells(data).cells, [data])
  const maximum = metricMaximum(data, metric)
  const hasActivity = flatCells.some((fc) => Boolean(fc.cell?.in_range && fc.cell.request_count > 0))
  const { daysPerRow, cellSize, needsHorizontalScroll } = useMemo(
    () => computeLayout(containerWidth || 1200),
    [containerWidth],
  )

  if (data.rows.length === 0) {
    return <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">No heatmap data</div>
  }

  const colsPerRow = 24 * daysPerRow

  const placeTooltip = (label: string, x: number, y: number) => {
    if (!label) return
    setTooltip({
      label,
      x: Math.max(x, 8),
      y: Math.max(y, 8),
    })
  }

  const showPointerTooltip = (event: MouseEvent<HTMLElement>, fc: FlatCell) => {
    placeTooltip(cellTooltipLabel(fc), event.clientX + 12, event.clientY + 12)
  }
  const showFocusTooltip = (event: FocusEvent<HTMLElement>, fc: FlatCell) => {
    const rect = event.currentTarget.getBoundingClientRect()
    placeTooltip(cellTooltipLabel(fc), rect.left + rect.width + 8, rect.top + rect.height + 8)
  }
  const showClickTooltip = (event: MouseEvent<HTMLElement>, fc: FlatCell) => {
    const rect = event.currentTarget.getBoundingClientRect()
    placeTooltip(cellTooltipLabel(fc), rect.left + rect.width + 8, rect.top + rect.height + 8)
  }
  const hideTooltip = () => setTooltip(null)

  const rows: Array<{ label: HeatmapRowLabel; cells: FlatCell[] }> = []
  for (let i = 0; i < flatCells.length; i += colsPerRow) {
    const rowCells = flatCells.slice(i, i + colsPerRow)
    const firstCell = rowCells[0]
    rows.push({
      label: firstCell ? splitRowLabel(firstCell.dateLabel, firstCell.date) : { dateLabel: "", weekdayLabel: "" },
      cells: rowCells,
    })
  }

  const daySep = 3
  const gridCols: string[] = [`${heatmapLabelWidth}px`]
  for (let d = 0; d < daysPerRow; d++) {
    if (d > 0) gridCols.push(`${daySep}px`)
    for (let h = 0; h < 24; h++) gridCols.push(`${cellSize}px`)
  }
  const gridTemplateColumns = gridCols.join(" ")
  const metricLabel = metricOptions.find((option) => option.value === metric)?.label ?? "Tokens"

  return (
    <div ref={containerRef}>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div className="inline-flex rounded-md bg-muted/60 p-0.5" role="group" aria-label="Heatmap metric">
          {metricOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              aria-pressed={metric === option.value}
              className={`rounded px-2 py-1 text-xs font-medium transition-colors ${metric === option.value ? "bg-background text-foreground shadow-xs" : "text-muted-foreground hover:text-foreground"}`}
              onClick={() => { setMetric(option.value); hideTooltip() }}
            >
              {option.label}
            </button>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground" aria-label={`${metricLabel} legend`}>
          <span className="inline-flex items-center gap-1">
            <span
              aria-hidden="true"
              className="h-2.5 w-8 rounded-[2px]"
              style={{ backgroundImage: `linear-gradient(90deg, ${metricColor(metric, 0.12)}, ${metricColor(metric, 0.95)})` }}
            />
            {metricLabel}: 0–{formatCompact(maximum, 1)}
          </span>
          <span className="inline-flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-[2px] bg-muted/40" />No activity</span>
          <span className="inline-flex items-center gap-1"><span className="relative h-2.5 w-2.5 rounded-[2px] bg-terracotta-500/10"><span className="absolute inset-0 m-auto h-[2px] w-[2px] rounded-full bg-foreground/45" /></span>Zero</span>
          <span className="inline-flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-[2px] bg-muted/10 ring-1 ring-border/30" />Outside range</span>
          {metric === "tokens" ? (
            <>
              <span className="inline-flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-[2px] border border-terracotta-400 bg-[repeating-linear-gradient(135deg,transparent_0_2px,rgba(217,119,87,0.35)_2px_3px)]" />Partial</span>
              <span className="inline-flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-[2px] bg-[repeating-linear-gradient(45deg,rgba(113,113,122,0.15)_0_2px,rgba(113,113,122,0.45)_2px_3px)]" />Unavailable</span>
            </>
          ) : null}
        </div>
      </div>

      {metric === "failures" && hasActivity && maximum === 0 ? <p className="mb-2 text-xs text-muted-foreground" role="status">No failures in this period</p> : null}

      {needsHorizontalScroll ? (
        <p className="mb-1 flex items-center justify-end gap-1 text-[11px] text-muted-foreground" role="note">
          Swipe for all hours <span aria-hidden="true">→</span>
        </p>
      ) : null}

      <div className="relative overflow-x-auto pb-1">
        <div className="w-max min-w-full space-y-2">
          {rows.map((row, rowIdx) => (
            <div key={`row-${rowIdx}`} className="grid gap-[1px]" style={{ gridTemplateColumns }}>
              <div className="sticky left-0 z-20 grid h-full grid-cols-[5ch_3ch] items-center gap-1 border-r border-border/30 bg-card pr-2 text-[10px] font-medium text-muted-foreground/60">
                <span className="text-right tabular-nums">{row.label.dateLabel}</span>
                <span className="text-left">{row.label.weekdayLabel}</span>
              </div>

              {row.cells.map((fc, cellIndex) => {
                const datum = heatmapCellDatum(fc.cell, metric, maximum)
                const label = cellTooltipLabel(fc)
                const isPartial = datum.state === "token-partial"
                const isUnavailable = datum.state === "token-unavailable"
                const style = datum.state === "out-of-range"
                  ? { backgroundColor: "rgba(113, 113, 122, 0.04)" }
                  : datum.state === "no-activity"
                    ? { backgroundColor: "rgba(113, 113, 122, 0.12)" }
                    : isUnavailable
                      ? { backgroundImage: "repeating-linear-gradient(45deg, rgba(113,113,122,0.12) 0 2px, rgba(113,113,122,0.42) 2px 3px)" }
                      : {
                          backgroundColor: metricColor(metric, 0.12 + datum.intensity * 0.83),
                          backgroundImage: isPartial ? "repeating-linear-gradient(135deg, transparent 0 3px, rgba(255,255,255,0.55) 3px 4px)" : undefined,
                        }
                const cell = label ? (
                  <button
                    type="button"
                    aria-label={label}
                    data-state={datum.state}
                    className="group relative aspect-square rounded-[2px] border border-transparent transition-all duration-200 hover:border-terracotta-300 hover:shadow-xs md:hover:z-10 md:hover:scale-150 md:hover:rounded-sm focus-visible:z-10 focus-visible:outline-hidden focus-visible:ring-1 focus-visible:ring-terracotta-500"
                    style={style}
                    onMouseEnter={(event) => showPointerTooltip(event, fc)}
                    onMouseMove={(event) => showPointerTooltip(event, fc)}
                    onMouseLeave={(event) => {
                      // Loading adjacent sections can move a clicked cell out
                      // from under the pointer while it still owns focus.
                      if (event.currentTarget !== document.activeElement) hideTooltip()
                    }}
                    onFocus={(event) => showFocusTooltip(event, fc)}
                    onBlur={hideTooltip}
                    onClick={(event) => showClickTooltip(event, fc)}
                    onKeyDown={(event) => {
                      if (event.key === "Escape") hideTooltip()
                    }}
                  >
                    {datum.hasActivity && datum.value === 0 && !isUnavailable ? <span className="absolute inset-0 m-auto block h-[2px] w-[2px] rounded-full bg-foreground/45" /> : null}
                    {isPartial ? <span className="sr-only">Partial token coverage</span> : null}
                  </button>
                ) : (
                  <div
                    aria-label={`${fc.dateLabel} ${fc.hour.toString().padStart(2, "0")}:00 · Outside selected range`}
                    data-state="out-of-range"
                    className="aspect-square rounded-[2px]"
                    style={style}
                  />
                )

                return (
                  <Fragment key={`${fc.date}-${fc.hour}`}>
                    {cellIndex > 0 && cellIndex % 24 === 0 ? <div className="rounded-full bg-border/20" style={{ width: `${daySep - 1}px`, marginLeft: "1px" }} /> : null}
                    {cell}
                  </Fragment>
                )
              })}
            </div>
          ))}
        </div>
      </div>

      {tooltip ? (
        <div ref={tooltipRef} role="tooltip" className="pointer-events-none fixed z-50 max-w-[min(320px,calc(100vw-16px))] rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-lg" style={{ left: tooltip.x, top: tooltip.y }}>
          {tooltip.label}
        </div>
      ) : null}
    </div>
  )
}
