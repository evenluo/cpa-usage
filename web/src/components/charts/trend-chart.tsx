import {
  ComposedChart,
  Area,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts"
import { useMemo } from "react"
import type { Formatter, NameType, ValueType } from "recharts/types/component/DefaultTooltipContent"
import type { ModelDistribution, TrendPoint, TimeGranularity } from "@/types/api"
import { formatCost, formatCompact } from "@/lib/format"
import {
  buildTrendSeriesConfig,
  buildTrendTickFormatter,
  mapTrendChartRows,
  type TrendChartMode,
} from "@/features/usage-intelligence/trend-model"

interface TrendChartProps {
  data: TrendPoint[]
  granularity: TimeGranularity
  mode?: TrendChartMode
}

export function TrendChart({ data, granularity, mode = "cost-token" }: TrendChartProps) {
  const {
    primaryKey,
    primaryName,
    primaryColor,
    gradientId,
    overlayKey,
    overlayName,
    tokenSeries,
  } = buildTrendSeriesConfig(mode)

  const chartData = useMemo(() => mapTrendChartRows(data), [data])
  const formatTick = useMemo(
    () => buildTrendTickFormatter(data.map((p) => p.label), granularity),
    [data, granularity]
  )
  const tooltipFormatter: Formatter<ValueType, NameType> = (value, name, item) => {
    if (name === "Cost") {
      const costStatus = item.payload?.costStatus
      if (costStatus === "unavailable") return ["Unavailable", "Cost"]
      return [formatCost(Number(value)), "Cost"]
    }
    if (name === "Requests") {
      return [Number(value).toLocaleString("en"), "Requests"]
    }
    return [`${formatCompact(Number(value), 2)} tokens`, String(name)]
  }

  return (
    <ResponsiveContainer width="100%" height="100%">
      <ComposedChart data={chartData} margin={{ top: 10, right: 10, bottom: 0, left: 0 }}>
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
          tickFormatter={(label: string) => formatTick(label)}
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
          hide={mode === "tokens"}
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
        {mode === "tokens" && (
          <Legend
            verticalAlign="top"
            align="right"
            iconType="line"
            wrapperStyle={{ fontSize: "12px", paddingBottom: "8px" }}
          />
        )}
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
        {mode === "tokens" ? (
          tokenSeries.map((series) => (
            <Line
              key={series.key}
              yAxisId="primary"
              type="monotone"
              dataKey={series.key}
              name={series.name}
              stroke={series.color}
              strokeWidth={1.75}
              dot={false}
              activeDot={{ r: 3, fill: series.color, stroke: "#fff", strokeWidth: 2 }}
            />
          ))
        ) : (
          <Line
            yAxisId="tokens"
            type="monotone"
            dataKey={overlayKey}
            name={overlayName}
            stroke="#94a3b8"
            strokeWidth={1.5}
            strokeDasharray="5 5"
            dot={false}
            activeDot={{ r: 3, fill: "#94a3b8", stroke: "#fff", strokeWidth: 2 }}
          />
        )}
      </ComposedChart>
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
      acc.output += row.output_tokens
      acc.reasoning += row.reasoning_tokens
      acc.cached += row.cached_tokens
      return acc
    },
    { total: 0, input: 0, output: 0, reasoning: 0, cached: 0 }
  )
  const maxValue = Math.max(totals.total, totals.input, totals.output, totals.reasoning, totals.cached, 1)
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
    { label: "Output", value: totals.output, color: "bg-emerald-500" },
    { label: "Reasoning", value: totals.reasoning, color: "bg-violet-500" },
    { label: "Cached", value: totals.cached, color: "bg-amber-500" },
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
