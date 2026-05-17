import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useEvents } from "@/hooks/useEvents"
import { formatCompact, formatDate } from "@/lib/format"

export function RequestEvidence() {
  const { data, isLoading, error } = useEvents("24h", 10)
  const events = data?.events.slice(0, 6) ?? []

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4 pb-2">
        <div>
          <CardTitle>Request Evidence</CardTitle>
          <CardDescription>Recent request samples supporting the health view</CardDescription>
        </div>
        <Badge variant="outline">Last 24h</Badge>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 6 }).map((_, index) => (
              <Skeleton key={index} className="h-20 w-full" />
            ))}
          </div>
        ) : error ? (
          <div className="flex h-24 items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
            Failed to load request evidence
          </div>
        ) : events.length === 0 ? (
          <div className="flex h-24 items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No recent request evidence
          </div>
        ) : (
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            {events.map((event) => (
              <div
                key={`${event.id ?? event.timestamp}-${event.auth_index ?? event.source}`}
                className="min-w-0 rounded-lg border border-border p-3"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{event.model || "Unknown model"}</p>
                    <p className="truncate text-xs text-muted-foreground">
                      {event.auth_index || event.source || "No key trace"}
                    </p>
                  </div>
                  <Badge variant={event.failed ? "amber" : "green"} className="shrink-0 text-[10px]">
                    {event.failed ? "Failed" : "Success"}
                  </Badge>
                </div>
                <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                  <span>{formatCompact(event.tokens?.total_tokens ?? 0, 2)} tokens</span>
                  <span>{event.latency_ms > 0 ? `${event.latency_ms}ms` : "No latency"}</span>
                  <span className="truncate">{formatDate(event.timestamp)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
