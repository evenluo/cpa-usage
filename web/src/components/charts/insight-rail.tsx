import type { Insight } from "@/types/api"
import { Badge } from "@/components/ui/badge"
import { formatCost, formatCompact } from "@/lib/format"

interface InsightRailProps {
  insights: Insight[]
}

function insightPriority(type: string): number {
  switch (type) {
    case "metric_completeness": return 0
    case "failure_concentration": return 1
    case "cache_efficiency": return 2
    case "top_cost_key": return 3
    case "token_spike": return 4
    default: return 5
  }
}

function formatMetric(insight: Insight): string {
  switch (insight.metric_label) {
    case "Cost": return formatCost(insight.metric_value)
    case "Tokens": return `${formatCompact(insight.metric_value, 2)} tokens`
    case "Failures": return `${insight.count.toLocaleString("en")} failures`
    case "Share": return `${insight.metric_value.toFixed(1)}% token share`
    case "Cache Read Share": return `${insight.metric_value.toFixed(1)}%`
    case "Metric Completeness": return insight.subject
    case "Cache state": return insight.subject
    case "Cost status": return `Cost ${insight.cost_status}`
    default: return `${insight.metric_label}: ${formatCompact(insight.metric_value, 2)}`
  }
}

const severityStyles = {
  green: "bg-emerald-50 text-emerald-700 border-emerald-200",
  blue: "bg-blue-50 text-blue-700 border-blue-200",
  violet: "bg-violet-50 text-violet-700 border-violet-200",
  amber: "bg-amber-50 text-amber-700 border-amber-200",
}

export function InsightRail({ insights }: InsightRailProps) {
  if (insights.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
        No deterministic insights
      </div>
    )
  }

  const ordered = [...insights].sort(
    (a, b) => insightPriority(a.type) - insightPriority(b.type)
  )

  return (
    <div className="space-y-3">
      {ordered.map((insight) => (
        <article
          key={insight.type}
          className="rounded-lg border border-border bg-card p-3.5"
          data-insight-type={insight.type}
        >
          <div className="flex items-start justify-between gap-3">
            <Badge
              variant="outline"
              className={severityStyles[insight.severity]}
            >
              {insight.title}
            </Badge>
            <span className="text-xs font-medium text-muted-foreground">
              {formatMetric(insight)}
            </span>
          </div>
          <p className="mt-2.5 text-sm font-medium">{insight.subject}</p>
          <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
            {insight.detail}
          </p>
        </article>
      ))}
    </div>
  )
}
