import type { HeatmapCell, HeatmapData } from "@/types/api"

export interface FlatCell {
  date: string
  dateLabel: string
  hour: number
  cell: HeatmapCell | null
}

export interface HeatmapRowLabel {
  dateLabel: string
  weekdayLabel: string
}

export type HeatmapMetric = "tokens" | "attempts" | "failures"

export type HeatmapCellState =
  | "out-of-range"
  | "no-activity"
  | "token-unavailable"
  | "token-partial"
  | "observed"

export interface HeatmapCellDatum {
  state: HeatmapCellState
  value: number
  intensity: number
  hasActivity: boolean
}

export const heatmapLabelWidth = 68

function fallbackDateLabel(date: string): string {
  const match = date.match(/^\d{4}-(\d{2})-(\d{2})$/)
  return match ? `${match[1]}/${match[2]}` : date
}

export function splitRowLabel(label: string, date: string): HeatmapRowLabel {
  const dateLabel = label.match(/\d{1,2}\/\d{1,2}/)?.[0] ?? fallbackDateLabel(date)
  const weekdayLabel = label.match(/[A-Za-z]{3,}/)?.[0] ?? ""
  return { dateLabel, weekdayLabel }
}

export function flattenCells(data: HeatmapData): {
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

export function metricMaximum(data: HeatmapData, metric: HeatmapMetric): number {
  if (metric === "attempts") return data.max_requests
  if (metric === "failures") return data.max_failures
  return data.max_tokens
}

export function metricValue(cell: HeatmapCell, metric: HeatmapMetric): number {
  if (metric === "attempts") return cell.request_count
  if (metric === "failures") return cell.failure_count
  return cell.total_tokens
}

export function heatmapCellDatum(
  cell: HeatmapCell | null,
  metric: HeatmapMetric,
  maximum: number,
): HeatmapCellDatum {
  if (!cell || !cell.in_range) {
    return { state: "out-of-range", value: 0, intensity: 0, hasActivity: false }
  }

  const hasActivity = cell.request_count > 0
  if (!hasActivity) {
    return { state: "no-activity", value: 0, intensity: 0, hasActivity: false }
  }

  const value = metricValue(cell, metric)
  const intensity = maximum > 0 ? Math.min(value / maximum, 1) : 0
  if (metric === "tokens" && cell.canonical_valid_attempts === 0) {
    return { state: "token-unavailable", value, intensity: 0, hasActivity }
  }
  if (
    metric === "tokens" &&
    cell.canonical_valid_attempts < cell.request_count
  ) {
    return { state: "token-partial", value, intensity, hasActivity }
  }
  return { state: "observed", value, intensity, hasActivity }
}

export interface HeatmapLayout {
  daysPerRow: number
  cellSize: number
  needsHorizontalScroll: boolean
}

export function computeLayout(containerWidth: number): HeatmapLayout {
  const hoursPerDay = 24
  const gap = 1
  const daySep = 3
  const minCellSize = 8
  const mobileCellSize = 24

  if (containerWidth < 768) {
    const gridWidth = heatmapLabelWidth + hoursPerDay * mobileCellSize + hoursPerDay * gap
    return {
      daysPerRow: 1,
      cellSize: mobileCellSize,
      needsHorizontalScroll: gridWidth > containerWidth,
    }
  }

  for (let daysPerRow = 3; daysPerRow >= 1; daysPerRow--) {
    const cols = hoursPerDay * daysPerRow
    const totalGap = (cols - 1) * gap + (daysPerRow - 1) * daySep
    const cellSize = Math.floor((containerWidth - heatmapLabelWidth - totalGap) / cols)
    if (cellSize >= minCellSize) {
      return { daysPerRow, cellSize, needsHorizontalScroll: false }
    }
  }

  return { daysPerRow: 1, cellSize: mobileCellSize, needsHorizontalScroll: true }
}
