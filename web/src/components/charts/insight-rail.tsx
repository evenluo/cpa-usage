import type { Insight } from "@/types/api"
import { AlertTriangle, CircleAlert, CircleDollarSign, Flame, type LucideIcon } from "lucide-react"
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

function insightIcon(type: string): LucideIcon {
  switch (type) {
    case "metric_completeness": return CircleDollarSign
    case "failure_concentration": return Flame
    default: return CircleAlert
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
      className="overflow-hidden rounded-xl border border-amber-200/80 bg-gradient-to-br from-amber-50/90 via-amber-50/40 to-transparent shadow-[0_1px_2px_rgba(146,64,14,0.06)] dark:border-amber-900/70 dark:from-amber-950/40 dark:via-amber-950/15 dark:to-transparent"
    >
      <div className="grid md:grid-cols-[240px_minmax(0,1fr)]">
        <div className="flex items-start gap-3 border-b border-amber-200/80 px-4 py-4 md:border-b-0 md:border-r dark:border-amber-900/70">
          <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-amber-300/60 bg-amber-100/80 dark:border-amber-800/60 dark:bg-amber-900/40">
            <AlertTriangle className="h-4 w-4 text-amber-700 dark:text-amber-400" aria-hidden="true" />
          </span>
          <div>
            <div className="flex items-center gap-2">
              <h3 id="attention-heading" className="font-serif text-base font-semibold text-amber-950 dark:text-amber-100">
                Needs attention
              </h3>
              <span className="rounded-full bg-amber-500/15 px-1.5 py-px text-[10px] font-semibold leading-4 tabular-nums text-amber-800 dark:text-amber-300">
                {ordered.length}
              </span>
            </div>
            <p className="mt-1 text-xs leading-relaxed text-amber-800/80 dark:text-amber-300/80">
              Warnings that affect how this window should be interpreted.
            </p>
          </div>
        </div>
        <div className="grid divide-y divide-amber-200/80 sm:grid-cols-2 sm:divide-x sm:divide-y-0 dark:divide-amber-900/70">
          {ordered.map((insight) => {
            const Icon = insightIcon(insight.type)
            return (
              <article
                key={insight.type}
                className="min-w-0 px-4 py-3.5"
                data-insight-type={insight.type}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-center gap-2">
                    <Icon className="h-3.5 w-3.5 shrink-0 text-amber-700/90 dark:text-amber-400/90" aria-hidden="true" />
                    <p className="truncate text-sm font-semibold text-foreground">{insight.title}</p>
                  </div>
                  <span className="shrink-0 rounded-full border border-amber-300/50 bg-background/60 px-2 py-px text-[11px] font-medium leading-4 tabular-nums text-amber-800 dark:border-amber-800/50 dark:bg-amber-950/30 dark:text-amber-300">
                    {formatMetric(insight)}
                  </span>
                </div>
                <p className="mt-1.5 pl-[22px] text-sm font-medium text-foreground/80">{insight.subject}</p>
                <p className="mt-1 pl-[22px] text-xs leading-relaxed text-muted-foreground">{insight.detail}</p>
              </article>
            )
          })}
        </div>
      </div>
    </section>
  )
}
