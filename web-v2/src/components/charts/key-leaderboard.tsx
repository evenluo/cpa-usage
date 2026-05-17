import type { KeyAliasBreakdown } from "@/types/api"
import { formatCost, formatCompact } from "@/lib/format"

interface KeyLeaderboardProps {
  data: KeyAliasBreakdown[]
  measure: "cost" | "tokens"
}

export function KeyLeaderboard({ data, measure }: KeyLeaderboardProps) {
  const sorted = [...data].sort(
    (a, b) =>
      (measure === "cost" ? b.total_cost : b.total_tokens) -
      (measure === "cost" ? a.total_cost : a.total_tokens)
  )
  const rows = sorted.slice(0, 5)
  const total = data.reduce(
    (sum, row) => sum + (measure === "cost" ? row.total_cost : row.total_tokens),
    0
  )

  if (rows.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
        No key alias usage in this range
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {rows.map((row, i) => {
        const value = measure === "cost" ? row.total_cost : row.total_tokens
        const pct = total > 0 ? (value / total) * 100 : 0
        const label = row.alias || row.traceability || row.identity

        return (
          <div
            key={row.identity}
            className="flex items-center gap-3 rounded-lg border border-border p-2.5"
          >
            <span className="w-5 text-xs font-semibold text-muted-foreground">
              #{i + 1}
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{label}</p>
              <p className="truncate text-[11px] text-muted-foreground">
                {row.identity}
              </p>
            </div>
            <div className="text-right">
              <p className="text-sm font-semibold">
                {measure === "cost" && row.cost_available
                  ? formatCost(value)
                  : `${formatCompact(value, 1)} tokens`}
              </p>
              <p className="text-[11px] text-muted-foreground">
                {pct.toFixed(1)}% of total
              </p>
            </div>
          </div>
        )
      })}
    </div>
  )
}
