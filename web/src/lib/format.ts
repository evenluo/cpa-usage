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

export function formatComparison(
  value: number | null | undefined,
  unit: "%" | "pp",
): string {
  if (value === null || value === undefined) return "No previous data"
  const sign = value > 0 ? "+" : ""
  return `${sign}${value.toFixed(1)}${unit} vs previous`
}
