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
import type { TrendPoint, TimeGranularity } from "@/types/api"
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
