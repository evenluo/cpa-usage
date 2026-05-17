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
import type { TrendPoint, TimeRange } from "@/types/api"
import { formatCost, formatCompact } from "@/lib/format"

interface TrendChartProps {
  data: TrendPoint[]
  range?: TimeRange
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

export function TrendChart({ data, range }: TrendChartProps) {
  const chartData = data.map((p) => ({
    label: p.label,
    cost: p.cost_status === "unavailable" ? null : p.total_cost,
    tokens: p.total_tokens,
    costStatus: p.cost_status,
  }))

  return (
    <ResponsiveContainer width="100%" height="100%">
      <AreaChart data={chartData} margin={{ top: 10, right: 10, bottom: 0, left: 0 }}>
        <defs>
          <linearGradient id="costGradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#d97757" stopOpacity={0.25} />
            <stop offset="100%" stopColor="#d97757" stopOpacity={0} />
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
          yAxisId="cost"
          orientation="left"
          tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }}
          tickFormatter={(v: number) => `$${formatCompact(v)}`}
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
          formatter={(value: number, name: string, props: { payload: { costStatus: string } }) => {
            if (name === "Cost") {
              if (props.payload.costStatus === "unavailable") return ["Unavailable", "Cost"]
              return [formatCost(value), "Cost"]
            }
            return [`${formatCompact(value, 2)} tokens`, "Tokens"]
          }}
          labelFormatter={(label) => label}
        />
        <Area
          yAxisId="cost"
          type="monotone"
          dataKey="cost"
          stroke="#d97757"
          strokeWidth={2}
          fill="url(#costGradient)"
          dot={false}
          activeDot={{ r: 4, fill: "#d97757", stroke: "#fff", strokeWidth: 2 }}
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
