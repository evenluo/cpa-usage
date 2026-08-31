import { Link } from "@tanstack/react-router"
import { ArrowRight } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { RequestEvidenceEvent } from "@/components/intelligence/request-evidence-event"
import type { UsageEventsPage } from "@/types/api"

interface RequestEvidenceProps {
  provider: string
  data: UsageEventsPage | undefined
  isLoading: boolean
  isRefreshing: boolean
  error: unknown
  onRetry: () => void
}

export function RequestEvidence({ provider, data, isLoading, isRefreshing, error, onRetry }: RequestEvidenceProps) {
  const latestEvent = data?.events[0]
  const hasCompleteData = data !== undefined

  return (
    <Card className="flex h-full min-w-0 flex-col overflow-hidden xl:h-[300px]">
      <CardHeader className="flex flex-row items-start justify-between gap-4 p-4 pb-3">
        <div className="min-w-0">
          <CardTitle className="text-base">Request Evidence</CardTitle>
          <CardDescription>Recent samples behind health</CardDescription>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {hasCompleteData && error ? (
            <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry refresh</Button>
          ) : null}
          <Badge variant="outline">Last 24h</Badge>
        </div>
      </CardHeader>
      <CardContent className="min-h-0 min-w-0 flex-1 p-4 pt-0">
        {!hasCompleteData && isLoading ? (
          <Skeleton className="h-[154px] w-full" />
        ) : !hasCompleteData && error ? (
          <div className="flex h-[180px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-sm text-red-500">
            <span>Failed to load request evidence</span>
            <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry request evidence</Button>
          </div>
        ) : !latestEvent ? (
          <div className="flex h-[180px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No recent request evidence
          </div>
        ) : (
          <div className="flex h-full min-w-0 flex-col justify-between gap-3">
            <RequestEvidenceEvent
              event={latestEvent}
              label="Latest upstream attempt"
              syncState={isRefreshing ? "refreshing" : "synced"}
            />
            <Link
              to="/requests"
              search={{ provider, model: "", result: "" }}
              className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg text-xs font-medium text-terracotta-700 transition-colors hover:bg-terracotta-500/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500 dark:text-terracotta-300"
            >
              View all attempts
              <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
            </Link>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
