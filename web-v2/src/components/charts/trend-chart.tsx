import {
  AreaChart,
  Area,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts"
import type { Formatter, NameType, ValueType } from "recharts/types/component/DefaultTooltipContent"
import type { ModelDistribution, TrendPoint, TimeRange } from "@/types/api"
import { formatCost, formatCompact } from "@/lib/format"

interface TrendChartProps {
  data: TrendPoint[]
  range?: TimeRange
  mode?: "cost-token" | "requests-token"
}

function formatTickLabel(label: string, range?: TimeRange): string {
  // Parse "2026-05-09 22:00 +0800" or similar
  const dateMatch = label.match(/(\d{4})-(\d{2})-(\d{2})/)
  if (!dateMatch) return label

  const [, year, month, day] = dateMatch
  const date = new Date(`${year}-${month}-${day}T00:00:00`)

  // For day granularity (30d range): show "May 9" or "5/9"
  if (range === "30d") {
    return date.toLocaleDateString("en", { month: "short", day: "numeric" })
  }

  // For 7d range: show date only on day boundaries, otherwise omit
  if (range === "7d") {
    const timeMatch = label.match(/(\d{2}):(\d{2})/)
    if (timeMatch && timeMatch[1] === "00") {
      return `${month}/${day}`
    }
    return "" // Skip non-midnight ticks to reduce clutter
  }

  // For 24h/today/yesterday: show hour only
  const timeMatch = label.match(/(\d{2}):(\d{2})/)
  if (timeMatch) {
    return `${timeMatch[1]}:00`
  }

  return label
}

export function TrendChart({ data, range, mode = "cost-token" }: TrendChartProps) {
  const primaryKey = mode === "cost-token" ? "cost" : "requests"
  const primaryName = mode === "cost-token" ? "Cost" : "Requests"
  const primaryColor = mode === "cost-token" ? "#d97757" : "#7c3aed"
  const gradientId = mode === "cost-token" ? "costGradient" : "requestGradient"

  const chartData = data.map((p) => ({
    label: p.label,
    cost: p.cost_status === "unavailable" ? null : p.total_cost,
    requests: p.request_count,
    tokens: p.total_tokens,
    costStatus: p.cost_status,
  }))
  const tooltipFormatter: Formatter<ValueType, NameType> = (value, name, item) => {
    if (name === "Cost") {
      const costStatus = item.payload?.costStatus
      if (costStatus === "unavailable") return ["Unavailable", "Cost"]
      return [formatCost(Number(value)), "Cost"]
    }
    if (name === "Requests") {
      return [Number(value).toLocaleString("en"), "Requests"]
    }
    return [`${formatCompact(Number(value), 2)} tokens`, "Tokens"]
  }

  return (
    <ResponsiveContainer width="100%" height="100%">
      <AreaChart data={chartData} margin={{ top: 10, right: 10, bottom: 0, left: 0 }}>
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={primaryColor} stopOpacity={0.25} />
            <stop offset="100%" stopColor={primaryColor} stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
        <XAxis
          dataKey="label"
          tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }}
          tickLine={false}
          axisLine={false}
          interval="preserveStartEnd"
          tickFormatter={(label: string) => formatTickLabel(label, range)}
        />
        <YAxis
          yAxisId="primary"
          orientation="left"
          tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }}
          tickFormatter={(v: number) =>
            mode === "cost-token" ? `$${formatCompact(v)}` : formatCompact(v)
          }
          tickLine={false}
          axisLine={false}
          width={60}
        />
        <YAxis
          yAxisId="tokens"
          orientation="right"
          tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }}
          tickFormatter={(v: number) => formatCompact(v)}
          tickLine={false}
          axisLine={false}
          width={60}
        />
        <Tooltip
          contentStyle={{
            backgroundColor: "hsl(var(--card))",
            border: "1px solid hsl(var(--border))",
            borderRadius: "0.5rem",
            fontSize: "12px",
            boxShadow: "0 4px 12px rgba(0,0,0,0.1)",
          }}
          formatter={tooltipFormatter}
          labelFormatter={(label) => label}
        />
        <Area
          yAxisId="primary"
          type="monotone"
          dataKey={primaryKey}
          name={primaryName}
          stroke={primaryColor}
          strokeWidth={2}
          fill={`url(#${gradientId})`}
          dot={false}
          activeDot={{ r: 4, fill: primaryColor, stroke: "#fff", strokeWidth: 2 }}
        />
        <Line
          yAxisId="tokens"
          type="monotone"
          dataKey="tokens"
          stroke="#94a3b8"
          strokeWidth={1.5}
          strokeDasharray="5 5"
          dot={false}
          activeDot={{ r: 3, fill: "#94a3b8", stroke: "#fff", strokeWidth: 2 }}
        />
      </AreaChart>
    </ResponsiveContainer>
  )
}

interface TokenBreakdownPanelProps {
  data: ModelDistribution[]
}

export function TokenBreakdownPanel({ data }: TokenBreakdownPanelProps) {
  const totals = data.reduce(
    (acc, row) => {
      acc.total += row.total_tokens
      acc.input += row.input_tokens
      acc.cached += row.cached_tokens
      return acc
    },
    { total: 0, input: 0, cached: 0 }
  )
  const outputEstimate = Math.max(totals.total - totals.input, 0)
  const maxValue = Math.max(totals.total, totals.input, totals.cached, outputEstimate, 1)
  const rows = [...data].sort((a, b) => b.total_tokens - a.total_tokens).slice(0, 5)

  if (data.length === 0) {
    return (
      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
        No token breakdown in this range
      </div>
    )
  }

  const breakdown = [
    { label: "Input", value: totals.input, color: "bg-blue-500" },
    { label: "Cached", value: totals.cached, color: "bg-amber-500" },
    { label: "Output", value: outputEstimate, color: "bg-emerald-500" },
  ]

  return (
    <div className="grid h-full gap-6 lg:grid-cols-[0.9fr_1.1fr]">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-medium uppercase text-muted-foreground">Total Tokens</p>
          <p className="mt-1 text-2xl font-semibold">{formatCompact(totals.total, 2)}</p>
        </div>
        <div className="space-y-3">
          {breakdown.map((item) => (
            <div key={item.label} className="space-y-1">
              <div className="flex items-center justify-between text-xs">
                <span className="text-muted-foreground">{item.label}</span>
                <span className="font-medium">{formatCompact(item.value, 2)}</span>
              </div>
              <div className="h-2 rounded-full bg-muted">
                <div
                  className={`h-full rounded-full ${item.color}`}
                  style={{ width: `${Math.max((item.value / maxValue) * 100, item.value > 0 ? 3 : 0)}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        {rows.map((row) => {
          const pct = totals.total > 0 ? (row.total_tokens / totals.total) * 100 : 0
          return (
            <div key={`${row.provider}-${row.model}`} className="rounded-lg border border-border p-3">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{row.model}</p>
                  <p className="text-xs text-muted-foreground">{row.provider || "Unknown"}</p>
                </div>
                <div className="text-right">
                  <p className="text-sm font-semibold">{formatCompact(row.total_tokens, 2)}</p>
                  <p className="text-xs text-muted-foreground">{pct.toFixed(1)}%</p>
                </div>
              </div>
              <div className="mt-2 h-1.5 rounded-full bg-muted">
                <div
                  className="h-full rounded-full bg-blue-500"
                  style={{ width: `${Math.max(pct, pct > 0 ? 3 : 0)}%` }}
                />
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
