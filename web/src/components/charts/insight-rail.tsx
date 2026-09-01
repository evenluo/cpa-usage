import type { Insight } from "@/types/api"
import { AlertTriangle } from "lucide-react"
import { formatCompact } from "@/lib/format"

interface InsightRailProps {
  insights: Insight[]
}

// Only warning-level (amber) insights reach this rail; today the backend emits
// exactly two (see internal/repository/analytics_insights.go).
function insightPriority(type: string): number {
  switch (type) {
    case "metric_completeness": return 0
    case "failure_concentration": return 1
    default: return 2
  }
}

function formatMetric(insight: Insight): string {
  switch (insight.metric_label) {
    case "Failures": return `${insight.count.toLocaleString("en")} failures`
    case "Metric Completeness": return insight.subject
    default: return `${insight.metric_label}: ${formatCompact(insight.metric_value, 2)}`
  }
}

export function InsightRail({ insights }: InsightRailProps) {
  const ordered = [...insights].sort(
    (a, b) => insightPriority(a.type) - insightPriority(b.type)
  )

  return (
    <section
      aria-labelledby="attention-heading"
      className="overflow-hidden rounded-lg border border-amber-200 bg-amber-50/60 dark:border-amber-900 dark:bg-amber-950/20"
    >
      <div className="grid md:grid-cols-[220px_minmax(0,1fr)]">
        <div className="flex items-start gap-3 border-b border-amber-200 px-4 py-4 md:border-b-0 md:border-r dark:border-amber-900">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-700 dark:text-amber-400" aria-hidden="true" />
          <div>
            <h3 id="attention-heading" className="font-serif text-base font-semibold text-amber-950 dark:text-amber-100">
              Needs attention
            </h3>
            <p className="mt-1 text-xs leading-relaxed text-amber-800/80 dark:text-amber-300/80">
              Warnings that affect how this window should be interpreted.
            </p>
          </div>
        </div>
        <div className="grid divide-y divide-amber-200 sm:grid-cols-2 sm:divide-x sm:divide-y-0 dark:divide-amber-900">
          {ordered.map((insight) => (
            <article key={insight.type} className="min-w-0 px-4 py-3.5" data-insight-type={insight.type}>
              <div className="flex items-start justify-between gap-3">
                <p className="text-sm font-semibold text-foreground">{insight.title}</p>
                <span className="shrink-0 text-xs font-medium text-amber-800 dark:text-amber-300">
                  {formatMetric(insight)}
                </span>
              </div>
              <p className="mt-1 text-sm font-medium text-foreground/80">{insight.subject}</p>
              <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{insight.detail}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  )
}
