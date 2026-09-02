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
  valueAvailable: boolean
  totalCost: number
  costAvailable: boolean
  totalTokens: number
  requestCount: number
  color: string
}

const MAX_VISIBLE_MODELS = 5
const PALETTE = ["#d97757", "#7f8f96", "#8d806f", "#6f8a7b", "#8b7f9c", "#a39b92"]
// Shorter than the recharts default so measure and time-window switches don't replay a slow sweep.
const MIX_ANIMATION_DURATION_MS = 500

type Measure = ModelDistributionProps["measure"]

interface MeasureConfig {
  value: (row: ModelDistribution) => number
  format: (value: number) => string
  emptyMessage: string
  // Shown when every visible row's value is unavailable rather than genuinely zero.
  unavailableMessage?: string
  centerLabel: string
  supportingMetric: (row: ModelMixRow) => string
}

const MEASURE_CONFIG: Record<Measure, MeasureConfig> = {
  cost: {
    value: (row) => row.total_cost,
    format: formatCost,
    emptyMessage: "No cost recorded for shown models",
    unavailableMessage: "Cost unavailable for shown models",
    centerLabel: "Shown cost mix",
    supportingMetric: (row) => `${formatCompact(row.totalTokens, 1)} tokens`,
  },
  tokens: {
    value: (row) => row.total_tokens,
    format: (value) => `${formatCompact(value, 1)} tokens`,
    emptyMessage: "No token usage recorded for shown models",
    centerLabel: "Shown token mix",
    supportingMetric: (row) => (row.costAvailable ? formatCost(row.totalCost) : "Cost n/a"),
  },
}

function hasAvailableCost(row: Pick<ModelDistribution, "cost_available" | "cost_status">): boolean {
  return row.cost_available && row.cost_status === "available"
}

// An unavailable cost must render as unavailable, never as a fabricated zero
// share, so such rows carry value 0 and are excluded from the mix total.
function measureRow(row: ModelDistribution, measure: Measure): Pick<ModelMixRow, "value" | "valueAvailable"> {
  const valueAvailable = measure === "tokens" || hasAvailableCost(row)
  return { value: valueAvailable ? MEASURE_CONFIG[measure].value(row) : 0, valueAvailable }
}

function buildRows(data: ModelDistribution[], measure: Measure): ModelMixRow[] {
  const config = MEASURE_CONFIG[measure]
  const sorted = [...data].sort((a, b) => {
    const difference = measureRow(b, measure).value - measureRow(a, measure).value
    return difference !== 0 ? difference : a.model.localeCompare(b.model)
  })
  const visible = sorted.slice(0, MAX_VISIBLE_MODELS).map((row, index) => ({
    key: `${row.provider}:${row.model}`,
    model: row.model,
    provider: row.provider || "Unknown provider",
    ...measureRow(row, measure),
    totalCost: row.total_cost,
    costAvailable: hasAvailableCost(row),
    totalTokens: row.total_tokens,
    requestCount: row.request_count,
    color: PALETTE[index],
  }))
  const remaining = sorted.slice(MAX_VISIBLE_MODELS)
  if (remaining.length === 0) return visible

  const otherCostAvailable = remaining.every(hasAvailableCost)
  const otherValueAvailable = measure === "tokens" || otherCostAvailable
  visible.push({
    key: "other-models",
    model: "Other shown models",
    provider: `${remaining.length} additional shown ${remaining.length === 1 ? "model" : "models"}`,
    value: otherValueAvailable ? remaining.reduce((sum, row) => sum + config.value(row), 0) : 0,
    valueAvailable: otherValueAvailable,
    totalCost: remaining.reduce((sum, row) => sum + row.total_cost, 0),
    costAvailable: otherCostAvailable,
    totalTokens: remaining.reduce((sum, row) => sum + row.total_tokens, 0),
    requestCount: remaining.reduce((sum, row) => sum + row.request_count, 0),
    color: PALETTE[PALETTE.length - 1],
  })
  return visible
}

function SupportingMetrics({ row, measure }: { row: ModelMixRow; measure: Measure }) {
  return (
    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
      <span>{row.provider}</span>
      <span>{MEASURE_CONFIG[measure].supportingMetric(row)}</span>
      <span>{formatCompact(row.requestCount, 0)} attempts</span>
    </div>
  )
}

export function ModelDistributionChart({ data, measure }: ModelDistributionProps) {
  const config = MEASURE_CONFIG[measure]
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
  if (total <= 0) {
    const message = config.unavailableMessage && rows.every((row) => !row.valueAvailable)
      ? config.unavailableMessage
      : config.emptyMessage
    return (
      <div className="flex min-h-[260px] items-center justify-center text-sm text-muted-foreground">
        {message}
      </div>
    )
  }

  const shareOf = (row: ModelMixRow) => (row.value / total) * 100
  const trailing = rows.slice(1)
  const twoColumnTrailing = trailing.length > 1
  // divide-y never underlines the final row; mirror that in grid mode by
  // dropping the bottom rule on the last visual row (the last item on mobile,
  // plus the second-to-last at sm+ when the count is even).
  const trailingRuleClass = (index: number) => {
    if (index === trailing.length - 1) return ""
    if (twoColumnTrailing && trailing.length % 2 === 0 && index === trailing.length - 2) {
      return "border-b border-border sm:border-b-0"
    }
    return "border-b border-border"
  }

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
              animationDuration={MIX_ANIMATION_DURATION_MS}
            >
              {rows.map((row) => (
                <Cell key={row.key} fill={row.color} strokeWidth={0} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
          <span className="font-serif text-3xl font-semibold tracking-tight">{data.length}</span>
          <span className="mt-0.5 text-xs text-muted-foreground">{data.length === 1 ? "model shown" : "models shown"}</span>
          <span className="mt-2 text-[11px] font-medium text-muted-foreground">{config.centerLabel}</span>
        </div>
      </div>

      <div className="min-w-0">
        <div className="border-b border-border pb-5">
          <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <span className="h-5 w-1 rounded-full" style={{ backgroundColor: leading.color }} aria-hidden="true" />
            Leading shown model
          </div>
          <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
            <p className="min-w-0 truncate font-serif text-2xl font-semibold tracking-tight sm:text-3xl">{leading.model}</p>
            <div className="shrink-0 sm:text-right">
              {leading.valueAvailable ? (
                <>
                  <p className="font-serif text-3xl font-semibold tracking-tight text-terracotta-700 dark:text-terracotta-400">
                    {shareOf(leading).toFixed(1)}%
                  </p>
                  <p className="text-xs font-medium text-muted-foreground">{config.format(leading.value)}</p>
                </>
              ) : (
                <p className="font-serif text-3xl font-semibold tracking-tight text-muted-foreground">Cost n/a</p>
              )}
            </div>
          </div>
          <SupportingMetrics row={leading} measure={measure} />
        </div>

        <div className={twoColumnTrailing ? "grid sm:grid-cols-2 sm:gap-x-8" : undefined}>
          {trailing.map((row, index) => (
            <div key={row.key} className={`grid gap-2 py-3.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:gap-6 ${trailingRuleClass(index)}`}>
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
                {row.valueAvailable ? (
                  <>
                    <p className="text-sm font-semibold">{shareOf(row).toFixed(1)}%</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">{config.format(row.value)}</p>
                  </>
                ) : (
                  <p className="text-sm font-semibold text-muted-foreground">Cost n/a</p>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
