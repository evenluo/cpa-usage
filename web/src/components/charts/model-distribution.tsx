import { Cell, Pie, PieChart, ResponsiveContainer } from "recharts"
import type { ModelDistribution } from "@/types/api"
import { formatCost, formatCompact } from "@/lib/format"

interface ModelDistributionProps {
  data: ModelDistribution[]
  measure: "cost" | "tokens"
}

interface ModelMixRow {
  key: string
  model: string
  provider: string
  value: number
  totalCost: number
  totalTokens: number
  requestCount: number
  color: string
}

const MAX_VISIBLE_MODELS = 5
const PALETTE = ["#d97757", "#7f8f96", "#8d806f", "#6f8a7b", "#8b7f9c", "#a39b92"]

function measureValue(row: ModelDistribution, measure: ModelDistributionProps["measure"]): number {
  return measure === "cost" ? row.total_cost : row.total_tokens
}

function formatMeasure(value: number, measure: ModelDistributionProps["measure"]): string {
  return measure === "cost" ? formatCost(value) : `${formatCompact(value, 1)} tokens`
}

function buildRows(data: ModelDistribution[], measure: ModelDistributionProps["measure"]): ModelMixRow[] {
  const sorted = [...data].sort((a, b) => {
    const difference = measureValue(b, measure) - measureValue(a, measure)
    return difference !== 0 ? difference : a.model.localeCompare(b.model)
  })
  const visible = sorted.slice(0, MAX_VISIBLE_MODELS).map((row, index) => ({
    key: `${row.provider}:${row.model}`,
    model: row.model,
    provider: row.provider || "Unknown provider",
    value: measureValue(row, measure),
    totalCost: row.total_cost,
    totalTokens: row.total_tokens,
    requestCount: row.request_count,
    color: PALETTE[index],
  }))
  const remaining = sorted.slice(MAX_VISIBLE_MODELS)
  if (remaining.length === 0) return visible

  visible.push({
    key: "other-models",
    model: "Other models",
    provider: `${remaining.length} additional ${remaining.length === 1 ? "model" : "models"}`,
    value: remaining.reduce((sum, row) => sum + measureValue(row, measure), 0),
    totalCost: remaining.reduce((sum, row) => sum + row.total_cost, 0),
    totalTokens: remaining.reduce((sum, row) => sum + row.total_tokens, 0),
    requestCount: remaining.reduce((sum, row) => sum + row.request_count, 0),
    color: PALETTE[PALETTE.length - 1],
  })
  return visible
}

function SupportingMetrics({ row, measure }: { row: ModelMixRow; measure: ModelDistributionProps["measure"] }) {
  return (
    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
      <span>{row.provider}</span>
      {measure === "cost" ? <span>{formatCompact(row.totalTokens, 1)} tokens</span> : <span>{formatCost(row.totalCost)}</span>}
      <span>{formatCompact(row.requestCount, 0)} attempts</span>
    </div>
  )
}

export function ModelDistributionChart({ data, measure }: ModelDistributionProps) {
  const rows = buildRows(data, measure)
  const total = rows.reduce((sum, row) => sum + row.value, 0)
  const leading = rows[0]

  if (!leading) {
    return (
      <div className="flex min-h-[260px] items-center justify-center text-sm text-muted-foreground">
        No model usage in this range
      </div>
    )
  }

  const shareOf = (row: ModelMixRow) => total > 0 ? (row.value / total) * 100 : 0

  return (
    <div className="grid gap-8 py-1 lg:grid-cols-[minmax(240px,0.72fr)_minmax(0,1.55fr)] lg:items-center xl:gap-12">
      <div className="relative mx-auto h-[250px] w-full max-w-[320px]">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={rows}
              cx="50%"
              cy="50%"
              innerRadius={72}
              outerRadius={108}
              paddingAngle={2}
              dataKey="value"
              nameKey="model"
              startAngle={90}
              endAngle={-270}
              isAnimationActive={false}
            >
              {rows.map((row) => (
                <Cell key={row.key} fill={row.color} strokeWidth={0} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
          <span className="font-serif text-3xl font-semibold tracking-tight">{data.length}</span>
          <span className="mt-0.5 text-xs text-muted-foreground">{data.length === 1 ? "model" : "models"}</span>
          <span className="mt-2 text-[11px] font-medium text-muted-foreground">{measure === "cost" ? "Cost mix" : "Token mix"}</span>
        </div>
      </div>

      <div className="min-w-0">
        <div className="border-b border-border pb-5">
          <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <span className="h-5 w-1 rounded-full" style={{ backgroundColor: leading.color }} aria-hidden="true" />
            Leading model
          </div>
          <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
            <p className="min-w-0 truncate font-serif text-2xl font-semibold tracking-tight sm:text-3xl">{leading.model}</p>
            <div className="shrink-0 sm:text-right">
              <p className="font-serif text-3xl font-semibold tracking-tight text-terracotta-700 dark:text-terracotta-400">
                {shareOf(leading).toFixed(1)}%
              </p>
              <p className="text-xs font-medium text-muted-foreground">{formatMeasure(leading.value, measure)}</p>
            </div>
          </div>
          <SupportingMetrics row={leading} measure={measure} />
        </div>

        <div className="divide-y divide-border">
          {rows.slice(1).map((row) => (
            <div key={row.key} className="grid gap-2 py-3.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:gap-6">
              <div className="min-w-0">
                <div className="flex items-center gap-2.5">
                  <span className="h-7 w-1 shrink-0 rounded-full" style={{ backgroundColor: row.color }} aria-hidden="true" />
                  <p className="truncate text-sm font-medium">{row.model}</p>
                </div>
                <div className="pl-3.5">
                  <SupportingMetrics row={row} measure={measure} />
                </div>
              </div>
              <div className="pl-3.5 sm:min-w-[120px] sm:pl-0 sm:text-right">
                <p className="text-sm font-semibold">{shareOf(row).toFixed(1)}%</p>
                <p className="mt-0.5 text-xs text-muted-foreground">{formatMeasure(row.value, measure)}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
