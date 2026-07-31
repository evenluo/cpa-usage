import type { TimeGranularity, TrendPoint } from "@/types/api"

export type TrendChartMode = "cost-token" | "requests-token" | "tokens"

export interface TrendSeriesConfig {
  primaryKey: string
  primaryName: string
  primaryColor: string
  gradientId: string
  overlayKey: string
  overlayName: string
  tokenSeries: Array<{ key: string; name: string; color: string }>
}

export function buildTrendSeriesConfig(mode: TrendChartMode): TrendSeriesConfig {
  return {
    primaryKey: mode === "cost-token" ? "cost" : mode === "requests-token" ? "requests" : "tokens",
    primaryName: mode === "cost-token" ? "Cost" : mode === "requests-token" ? "Requests" : "Tokens",
    primaryColor: mode === "cost-token" ? "#d97757" : mode === "requests-token" ? "#7c3aed" : "#2563eb",
    gradientId: mode === "cost-token" ? "costGradient" : mode === "requests-token" ? "requestGradient" : "tokenGradient",
    overlayKey: mode === "tokens" ? "requests" : "tokens",
    overlayName: mode === "tokens" ? "Requests" : "Tokens",
    tokenSeries: [
      { key: "inputTokens", name: "Input", color: "#059669" },
      { key: "outputTokens", name: "Output", color: "#d97706" },
      { key: "reasoningTokens", name: "Reasoning", color: "#7c3aed" },
      { key: "cachedTokens", name: "Cached", color: "#0891b2" },
    ],
  }
}

export interface TrendChartRow {
  label: string
  cost: number | null
  requests: number
  tokens: number
  inputTokens: number
  outputTokens: number
  reasoningTokens: number
  cachedTokens: number
  costStatus: TrendPoint["cost_status"]
}

export function mapTrendChartRows(points: TrendPoint[]): TrendChartRow[] {
  return points.map((p) => ({
    label: p.label,
    cost: p.cost_status === "unavailable" ? null : p.total_cost,
    requests: p.request_count,
    tokens: p.total_tokens,
    inputTokens: p.input_tokens,
    outputTokens: p.output_tokens,
    reasoningTokens: p.reasoning_tokens,
    cachedTokens: p.cached_tokens,
    costStatus: p.cost_status,
  }))
}

export type TrendTickFormatter = (label: string) => string

const DATE_PATTERN = /(\d{4})-(\d{2})-(\d{2})/
const TIME_PATTERN = /(\d{2}):(\d{2})/

// Hourly trends spanning more than two calendar days (e.g. a 7d window at hour
// granularity) only label day boundaries to stay readable; shorter hourly
// windows (today, yesterday, 24h) label every hour.
const MAX_HOUR_LABELED_DAYS = 2

export function buildTrendTickFormatter(labels: string[], granularity: TimeGranularity): TrendTickFormatter {
  if (granularity === "day") return formatDayTickLabel
  const distinctDays = new Set(
    labels
      .map((label) => label.match(DATE_PATTERN)?.[0])
      .filter((day): day is string => Boolean(day)),
  ).size
  return distinctDays > MAX_HOUR_LABELED_DAYS ? formatDayBoundaryTickLabel : formatHourTickLabel
}

// Day granularity: show "May 9".
function formatDayTickLabel(label: string): string {
  const dateMatch = label.match(DATE_PATTERN)
  if (!dateMatch) return label
  const [, year, month, day] = dateMatch
  const date = new Date(`${year}-${month}-${day}T00:00:00`)
  return date.toLocaleDateString("en", { month: "short", day: "numeric" })
}

// Hourly trend across many days: show the date only on day boundaries,
// otherwise omit the tick to reduce clutter.
function formatDayBoundaryTickLabel(label: string): string {
  const dateMatch = label.match(DATE_PATTERN)
  if (!dateMatch) return label
  const [, , month, day] = dateMatch
  const timeMatch = label.match(TIME_PATTERN)
  if (timeMatch && timeMatch[1] === "00") {
    return `${month}/${day}`
  }
  return ""
}

// Hourly trend within a short window: show the hour only.
function formatHourTickLabel(label: string): string {
  if (!DATE_PATTERN.test(label)) return label
  const timeMatch = label.match(TIME_PATTERN)
  if (timeMatch) {
    return `${timeMatch[1]}:00`
  }
  return label
}
