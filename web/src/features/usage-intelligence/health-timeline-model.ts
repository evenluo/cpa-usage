import type { TrendPoint } from "@/types/api"

export interface HealthBlock {
  date: string
  label: string
  hour: number
  success: number
  failure: number
  rate: number
}

export function groupByDate(trend: TrendPoint[]): Map<string, HealthBlock[]> {
  const groups = new Map<string, HealthBlock[]>()

  for (const p of trend) {
    // Parse "2026-05-09 22:00 +0800" → date = "2026-05-09", hour = 22
    const match = p.label.match(/(\d{4}-\d{2}-\d{2})\s+(\d{2}):/)
    if (!match) continue
    const [, date, hourStr] = match
    const hour = parseInt(hourStr, 10)

    const success = Math.max(p.request_count - p.failure_count, 0)
    const rate = p.request_count > 0 ? (success / p.request_count) * 100 : 0

    // Use short label like "May 9" from the date
    const d = new Date(date + "T00:00:00")
    const shortLabel = d.toLocaleDateString("en", { month: "short", day: "numeric" })

    const block: HealthBlock = {
      date,
      label: shortLabel,
      hour,
      success,
      failure: p.failure_count,
      rate: Number(rate.toFixed(1)),
    }

    if (!groups.has(date)) {
      groups.set(date, [])
    }
    groups.get(date)!.push(block)
  }

  // Sort each group's blocks by hour
  for (const blocks of groups.values()) {
    blocks.sort((a, b) => a.hour - b.hour)
  }

  return groups
}

export function cellColor(rate: number, hasFailure: boolean): string {
  if (!hasFailure) return "bg-emerald-500"
  if (rate >= 99) return "bg-emerald-400"
  if (rate >= 95) return "bg-amber-400"
  return "bg-red-400"
}
