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

export function computeLayout(containerWidth: number): { daysPerRow: number; cellSize: number } {
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
