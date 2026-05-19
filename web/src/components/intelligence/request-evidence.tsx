import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useEvents } from "@/hooks/useEvents"
import { formatCompact, formatDate } from "@/lib/format"

export function RequestEvidence({ provider }: { provider: string }) {
  const { data, isLoading, error } = useEvents("24h", 10, provider)
  const events = data?.events.slice(0, 5) ?? []

  return (
    <Card className="h-full">
      <CardHeader className="flex flex-row items-start justify-between gap-4 p-4 pb-3">
        <div>
          <CardTitle className="text-base">Request Evidence</CardTitle>
          <CardDescription>Recent samples behind health</CardDescription>
        </div>
        <Badge variant="outline">Last 24h</Badge>
      </CardHeader>
      <CardContent className="p-4 pt-0">
        {isLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, index) => (
              <Skeleton key={index} className="h-[58px] w-full" />
            ))}
          </div>
        ) : error ? (
          <div className="flex h-[180px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
            Failed to load request evidence
          </div>
        ) : events.length === 0 ? (
          <div className="flex h-[180px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No recent request evidence
          </div>
        ) : (
          <div className="space-y-2">
            {events.map((event) => {
              const keyLabel =
                event.api_key_alias || event.api_key_display || event.source || event.auth_index || "No key trace"
              const keyTrace = [
                event.api_key_alias ? event.api_key_display : "",
                event.source || event.auth_index || "",
              ]
                .filter(Boolean)
                .join(" · ")
              return (
                <div
                  key={`${event.id ?? event.timestamp}-${event.auth_index ?? event.source}`}
                  className="min-w-0 rounded-lg border border-border bg-background/45 p-2.5"
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{keyLabel}</p>
                      <p className="truncate text-xs text-muted-foreground">{keyTrace}</p>
                    </div>
                    <Badge variant={event.failed ? "amber" : "green"} className="shrink-0 text-[10px]">
                      {event.failed ? "Failed" : "Success"}
                    </Badge>
                  </div>
                  <div className="mt-2 grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 text-xs text-muted-foreground">
                    <span className="truncate">{event.model || "Unknown model"}</span>
                    <span>
                      {formatCompact(event.tokens?.total_tokens ?? 0, 2)} tokens
                      {event.latency_ms > 0 ? ` · ${event.latency_ms}ms` : ""}
                    </span>
                    <span className="truncate text-right">{formatDate(event.timestamp)}</span>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
