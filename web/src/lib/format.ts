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

export function formatDate(date: string | null): string {
  if (!date) return "Never"
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(date))
}

export function formatComparison(
  value: number | null | undefined,
  unit: "%" | "pp",
): string {
  if (value === null || value === undefined) return "No previous data"
  const sign = value > 0 ? "+" : ""
  return `${sign}${value.toFixed(1)}${unit} vs previous`
}
