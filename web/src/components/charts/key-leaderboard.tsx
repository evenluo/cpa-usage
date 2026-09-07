import type { KeyAliasBreakdown } from "@/types/api"
import { formatCost, formatCompact } from "@/lib/format"

interface KeyLeaderboardProps {
  data: KeyAliasBreakdown[]
}

export function KeyLeaderboard({ data }: KeyLeaderboardProps) {
  const hasAvailableCost = (row: KeyAliasBreakdown) => row.cost_available && row.cost_status === "available"
  const sorted = [...data].sort((a, b) => {
    const aHasCost = hasAvailableCost(a)
    const bHasCost = hasAvailableCost(b)
    if (aHasCost !== bHasCost) return bHasCost ? 1 : -1
    if (aHasCost && bHasCost) return b.total_cost - a.total_cost
    const aTokens = a.canonical_valid_attempts > 0 ? a.total_tokens : 0
    const bTokens = b.canonical_valid_attempts > 0 ? b.total_tokens : 0
    return bTokens - aTokens
  })
  const rows = sorted.slice(0, 5)
  const totalCost = data.reduce((sum, row) => sum + (hasAvailableCost(row) ? row.total_cost : 0), 0)
  const totalTokens = data.reduce((sum, row) => sum + (row.canonical_valid_attempts > 0 ? row.total_tokens : 0), 0)

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
        const costPct = hasAvailableCost(row) && totalCost > 0 ? (row.total_cost / totalCost) * 100 : null
        const tokenPct = row.canonical_valid_attempts > 0 && totalTokens > 0 ? (row.total_tokens / totalTokens) * 100 : null
        const label = row.alias || row.label || row.traceability || row.identity

        return (
          <div
            key={row.identity}
            className="flex flex-col items-start gap-2 rounded-lg border border-border px-2.5 py-2 sm:flex-row sm:items-center sm:gap-3"
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
            <div className="w-full text-left sm:w-auto sm:min-w-[156px] sm:text-right">
              <div className="flex flex-wrap items-baseline gap-2 sm:justify-end">
                <span className="text-sm font-semibold">
                  {hasAvailableCost(row) ? formatCost(row.total_cost) : "Cost n/a"}
                </span>
                <span className="text-[11px] font-medium text-blue-700">
                  {row.canonical_valid_attempts > 0 ? `${formatCompact(row.total_tokens, 1)} canonical tokens` : "Tokens n/a"}
                </span>
              </div>
              <p className="text-[11px] text-muted-foreground">
                {costPct === null ? "cost n/a" : `${costPct.toFixed(1)}% cost`} · {formatCompact(row.request_count, 0)} attempts · {tokenPct === null ? "tokens n/a" : `${tokenPct.toFixed(1)}% tokens`}
              </p>
            </div>
          </div>
        )
      })}
    </div>
  )
}
