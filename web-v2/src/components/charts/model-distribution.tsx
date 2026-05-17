import { PieChart, Pie, Cell, ResponsiveContainer } from "recharts"
import type { ModelDistribution } from "@/types/api"
import { formatCost, formatCompact } from "@/lib/format"
import { Badge } from "@/components/ui/badge"

interface ModelDistributionProps {
  data: ModelDistribution[]
  measure: "cost" | "tokens"
}

const PALETTE = ["#d97757", "#7c3aed", "#059669", "#d97706", "#0891b2", "#be123c"]

export function ModelDistributionChart({ data, measure }: ModelDistributionProps) {
  const chartData = data.map((row, i) => ({
    name: row.model,
    value: measure === "cost" ? row.total_cost : row.total_tokens,
    color: PALETTE[i % PALETTE.length],
  }))

  const total = chartData.reduce((sum, d) => sum + d.value, 0)

  return (
    <div className="grid gap-6 md:grid-cols-[200px_1fr] xl:grid-cols-1 2xl:grid-cols-[200px_1fr]">
      <div className="h-[180px]">
        {chartData.length > 0 ? (
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={chartData}
                cx="50%"
                cy="50%"
                innerRadius={50}
                outerRadius={80}
                paddingAngle={2}
                dataKey="value"
              >
                {chartData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} strokeWidth={0} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
        ) : (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            No model data
          </div>
        )}
      </div>

      <div className="space-y-2">
        {data.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
            No model usage in this range
          </div>
        ) : (
          data.map((row, i) => {
            const pct = total > 0 ? ((measure === "cost" ? row.total_cost : row.total_tokens) / total) * 100 : 0
            return (
              <div key={row.model} className="flex items-center gap-3 rounded-lg border border-border p-3">
                <div className="h-8 w-1 rounded-full" style={{ backgroundColor: PALETTE[i % PALETTE.length] }} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{row.model}</p>
                  <p className="text-xs text-muted-foreground">
                    {row.provider || "Unknown"} · {formatCompact(row.tokens, 1)} tokens · {formatCompact(row.requests, 0)} requests
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-sm font-semibold">
                    {measure === "cost" && row.cost_available ? formatCost(row.total_cost) : `${formatCompact(row.total_tokens, 1)} tokens`}
                  </p>
                  <p className="text-xs text-muted-foreground">{pct.toFixed(1)}%</p>
                </div>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
