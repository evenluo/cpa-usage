export function formatCost(value: number): string {
  if (value === 0) return "$0.00"
  if (value < 1) return `$${value.toLocaleString("en", { maximumFractionDigits: 4, minimumFractionDigits: 2 })}`
  return `$${value.toLocaleString("en", { maximumFractionDigits: 2, minimumFractionDigits: 2 })}`
}

export function formatCompact(value: number, fractionDigits = 1): string {
  return Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: fractionDigits }).format(value)
}

export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

const dateTimePartsFormat = new Intl.DateTimeFormat("en", {
  month: "numeric",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
})

/** Compact timestamp like "9/11 11:00" — numeric month/day, 24h clock. */
export function formatDate(date: string | null): string {
  if (!date) return "Never"
  const parts = dateTimePartsFormat.formatToParts(new Date(date))
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((p) => p.type === type)?.value ?? ""
  return `${part("month")}/${part("day")} ${part("hour")}:${part("minute")}`
}

/** Compact age like "12m ago" / "2h ago" / "3d ago"; absolute date past 7 days. */
export function formatRelativeAge(date: string, now: number = Date.now()): string {
  const seconds = Math.max(0, Math.round((now - new Date(date).getTime()) / 1000))
  if (seconds < 60) return "just now"
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days <= 7) return `${days}d ago`
  return formatDate(date)
}

export function formatComparison(
  value: number | null | undefined,
  unit: "%" | "pp",
): string {
  if (value === null || value === undefined) return "No previous data"
  const sign = value > 0 ? "+" : ""
  return `${sign}${value.toFixed(1)}${unit} vs previous`
}
