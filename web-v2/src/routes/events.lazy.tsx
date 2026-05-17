import { createLazyFileRoute } from "@tanstack/react-router"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { useEvents } from "@/hooks/useEvents"
import { formatCompact, formatDate } from "@/lib/format"

export const Route = createLazyFileRoute("/events")({
  component: EventsPage,
})

function EventsPage() {
  const { data, isLoading } = useEvents()
  const events = data?.events ?? []

  return (
    <div className="animate-slide-up space-y-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Request Events</p>
          <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight">Events</h1>
        </div>
        <Badge variant="outline">Last 24h</Badge>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Request Event Inspection</CardTitle>
          <CardDescription>Recent request events with traceability</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {isLoading ? (
              <>
                <Skeleton className="h-14 w-full" />
                <Skeleton className="h-14 w-full" />
                <Skeleton className="h-14 w-full" />
              </>
            ) : events.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
                No request events in this range
              </div>
            ) : (
              events.map((event) => (
                <div
                  key={`${event.id ?? event.timestamp}-${event.auth_index ?? event.source}`}
                  className="grid items-center gap-3 rounded-lg border border-border p-3 sm:grid-cols-[1fr_1fr_auto_auto]"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{event.source || "Unknown source"}</p>
                    <p className="truncate text-xs text-muted-foreground">{event.auth_index || "No auth index"}</p>
                  </div>
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{event.model || "Unknown model"}</p>
                    <p className="text-xs text-muted-foreground">{formatDate(event.timestamp)}</p>
                  </div>
                  <Badge variant={event.failed ? "amber" : "green"}>
                    {event.failed ? "Failed" : "Success"}
                  </Badge>
                  <div className="text-right">
                    <p className="text-sm font-medium">{formatCompact(event.tokens?.total_tokens ?? 0, 2)} tokens</p>
                    <p className="text-xs text-muted-foreground">{event.latency_ms > 0 ? `${event.latency_ms}ms` : "No latency"}</p>
                  </div>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
