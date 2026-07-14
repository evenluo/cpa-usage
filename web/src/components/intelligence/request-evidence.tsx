import { Link } from "@tanstack/react-router"
import { ArrowRight } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { RequestEvidenceEvent } from "@/components/intelligence/request-evidence-event"
import { useEvents } from "@/hooks/useEvents"

interface RequestEvidenceProps {
  provider: string
  range?: string
}

export function RequestEvidence({ provider, range = "24h" }: RequestEvidenceProps) {
  const { data, isLoading, error } = useEvents(range, 1, provider)
  const latestEvent = data?.events[0]

  return (
    <Card className="flex h-full min-w-0 flex-col overflow-hidden xl:h-[300px]">
      <CardHeader className="flex flex-row items-start justify-between gap-4 p-4 pb-3">
        <div className="min-w-0">
          <CardTitle className="text-base">Request Evidence</CardTitle>
          <CardDescription>Recent samples behind health</CardDescription>
        </div>
        <Badge variant="outline" className="shrink-0">Last 24h</Badge>
      </CardHeader>
      <CardContent className="min-h-0 min-w-0 flex-1 p-4 pt-0">
        {isLoading ? (
          <Skeleton className="h-[154px] w-full" />
        ) : error ? (
          <div className="flex h-[180px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
            Failed to load request evidence
          </div>
        ) : !latestEvent ? (
          <div className="flex h-[180px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No recent request evidence
          </div>
        ) : (
          <div className="flex h-full min-w-0 flex-col justify-between gap-3">
            <RequestEvidenceEvent event={latestEvent} label="Latest request" />
            <Link
              to="/requests"
              className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg text-xs font-medium text-terracotta-700 transition-colors hover:bg-terracotta-500/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500 dark:text-terracotta-300"
            >
              View all requests
              <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
            </Link>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
