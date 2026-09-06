import { Link } from "@tanstack/react-router"
import { ArrowUpRight, Pin } from "lucide-react"
import { useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { formatCompact } from "@/lib/format"
import type { UsageFailureBreakdown, UsageFailureDistribution } from "@/types/api"

interface FailureDistributionProps {
  provider: string
  data: UsageFailureDistribution | undefined
  isLoading: boolean
  error: unknown
  onRetry: () => void
}

type BreakdownKey = "status" | "provider" | "account" | "model" | "endpoint"

const sections: Array<{ title: string; field: keyof Pick<UsageFailureDistribution, "categories" | "statuses" | "providers" | "accounts" | "models" | "endpoints">; key: BreakdownKey }> = [
  { title: "Status families", field: "categories", key: "status" },
  { title: "Exact statuses", field: "statuses", key: "status" },
  { title: "Providers", field: "providers", key: "provider" },
  { title: "Accounts", field: "accounts", key: "account" },
  { title: "Models", field: "models", key: "model" },
  { title: "Endpoints", field: "endpoints", key: "endpoint" },
]

export function FailureDistribution({ provider, data, isLoading, error, onRetry }: FailureDistributionProps) {
  const hasCompleteData = data !== undefined

  return (
    <Card className="min-w-0 overflow-hidden">
      <CardHeader className="flex flex-col items-start justify-between gap-3 sm:flex-row">
        <div>
          <CardTitle className="flex items-center gap-2">
            Failure concentration
            <Pin className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Fixed 24-hour view" />
          </CardTitle>
          <CardDescription>Observed failed attempts by status and accountable dimensions; status is evidence, not a proven root cause.</CardDescription>
        </div>
        <div className="flex items-center gap-2">
          {hasCompleteData && error ? <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry refresh</Button> : null}
          <Badge variant="terracotta">24h fixed</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {!hasCompleteData && isLoading ? (
          <Skeleton className="h-40 w-full" />
        ) : !hasCompleteData && error ? (
          <div className="flex h-40 flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
            <span>Failed to load failure distribution</span>
            <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry failure distribution</Button>
          </div>
        ) : !data || data.total_failures === 0 ? (
          <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No failed attempts in the last 24 hours
          </div>
        ) : (
          <div className="space-y-4">
            <div className="flex items-baseline gap-2">
              <span className="font-serif text-2xl font-semibold">{formatCompact(data.total_failures)}</span>
              <span className="text-xs text-muted-foreground">failed attempts in scope</span>
            </div>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
              {sections.map((section) => (
                <FailureBreakdownSection
                  key={section.field}
                  title={section.title}
                  breakdown={data[section.field]}
                  selectionKey={section.key}
                  provider={provider}
                  windowEnd={data.window_end}
                />
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function FailureBreakdownSection({
  title,
  breakdown,
  selectionKey,
  provider,
  windowEnd,
}: {
  title: string
  breakdown: UsageFailureBreakdown
  selectionKey: BreakdownKey
  provider: string
  windowEnd: string
}) {
  const [expanded, setExpanded] = useState(false)
  const visibleItems = expanded ? breakdown.items : breakdown.items.slice(0, 4)
  const hiddenAttempts = breakdown.other_count + breakdown.items.slice(visibleItems.length).reduce((total, item) => total + item.count, 0)
  return (
    <section className="min-w-0" aria-label={title}>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
      <div className="space-y-1.5">
        {visibleItems.map((item) => {
          const search = {
            provider,
            model: "",
            modelAlias: "",
            account: "",
            endpoint: "",
            status: "",
            requestId: "",
            windowEnd,
            result: "failed" as const,
            [selectionKey]: item.value,
          }
          return (
            <Link
              key={item.value}
              to="/requests"
              search={search}
              aria-label={`Inspect ${item.label} failures`}
              className="flex min-w-0 items-center justify-between gap-2 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500"
            >
              <span className="truncate">{item.label}</span>
              <span className="flex shrink-0 items-center gap-1 font-medium">
                {formatCompact(item.count)}
                <ArrowUpRight className="h-3 w-3 text-muted-foreground" aria-hidden="true" />
              </span>
            </Link>
          )
        })}
        {expanded && breakdown.other_count > 0 ? (
          <div className="flex items-center justify-between gap-2 px-2 py-1.5 text-xs text-muted-foreground">
            <span>Other or unavailable</span>
            <span>{formatCompact(breakdown.other_count)}</span>
          </div>
        ) : null}
        {!expanded && hiddenAttempts > 0 ? (
          <Button type="button" variant="ghost" size="sm" className="h-auto w-full justify-between px-2 py-1.5 text-xs text-muted-foreground" onClick={() => setExpanded(true)}>
            <span>Show lower ranks</span>
            <span>{formatCompact(hiddenAttempts)} attempts</span>
          </Button>
        ) : null}
      </div>
    </section>
  )
}
