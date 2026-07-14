import { createLazyFileRoute, Link } from "@tanstack/react-router"
import { ArrowLeft, ChevronLeft, ChevronRight } from "lucide-react"
import { useState } from "react"
import {
  formatOutputTPS,
  getRequestEventLabels,
  RequestEvidenceEvent,
} from "@/components/intelligence/request-evidence-event"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useEvents } from "@/hooks/useEvents"
import { formatCompact, formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { UsageEvent } from "@/types/api"

const PAGE_SIZE = 10

export const Route = createLazyFileRoute("/requests")({
  component: RequestsPage,
})

function RequestsPage() {
  const [page, setPage] = useState(1)
  const [selectedEventKey, setSelectedEventKey] = useState<string | null>(null)
  const { data, isLoading, error } = useEvents("24h", PAGE_SIZE, "", page)
  const events = data?.events ?? []
  const totalPages = Math.max(data?.total_pages ?? 1, 1)
  const selectedEvent = events.find((event) => requestEventKey(event) === selectedEventKey) ?? events[0]
  const effectiveSelectedKey = selectedEvent ? requestEventKey(selectedEvent) : null

  function changePage(nextPage: number) {
    setSelectedEventKey(null)
    setPage(nextPage)
  }

  return (
    <div className="animate-slide-up mx-auto max-w-7xl space-y-6">
      <header className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-end">
        <div className="min-w-0">
          <Link
            to="/"
            className="mb-3 inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500"
          >
            <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
            Back to dashboard
          </Link>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Usage Intelligence
          </p>
          <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight">Request Evidence</h1>
          <p className="mt-1 text-sm text-muted-foreground">Recent request-level evidence behind service health.</p>
        </div>
        <Badge variant="outline" className="shrink-0">Last 24h</Badge>
      </header>

      {isLoading ? (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <Skeleton className="h-[420px] w-full" />
          <Skeleton className="h-[280px] w-full" />
        </div>
      ) : error ? (
        <Card>
          <CardContent className="flex min-h-[280px] items-center justify-center text-sm text-red-500">
            Failed to load request evidence
          </CardContent>
        </Card>
      ) : events.length === 0 ? (
        <Card>
          <CardContent className="flex min-h-[280px] items-center justify-center text-sm text-muted-foreground">
            No recent request evidence
          </CardContent>
        </Card>
      ) : (
        <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] lg:items-start">
          <Card className="min-w-0 overflow-hidden">
            <CardHeader className="flex flex-row items-start justify-between gap-4 p-4">
              <div className="min-w-0">
                <CardTitle>Recent requests</CardTitle>
                <CardDescription>Select a request to inspect its evidence.</CardDescription>
              </div>
              <Badge variant="outline" className="shrink-0">
                {formatCompact(data?.total_count ?? events.length)} total
              </Badge>
            </CardHeader>
            <CardContent className="space-y-3 p-4 pt-0">
              <div className="space-y-2">
                {events.map((event, index) => (
                  <RequestListItem
                    key={requestEventKey(event)}
                    event={event}
                    number={event.id ?? (page - 1) * PAGE_SIZE + index + 1}
                    selected={requestEventKey(event) === effectiveSelectedKey}
                    onSelect={() => setSelectedEventKey(requestEventKey(event))}
                  />
                ))}
              </div>

              <div className="flex items-center justify-between gap-3 border-t border-border pt-3">
                <Button
                  variant="outline"
                  size="sm"
                  aria-label="Previous page"
                  disabled={page <= 1}
                  onClick={() => changePage(page - 1)}
                >
                  <ChevronLeft className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
                  Previous
                </Button>
                <span className="whitespace-nowrap text-xs text-muted-foreground">Page {page} of {totalPages}</span>
                <Button
                  variant="outline"
                  size="sm"
                  aria-label="Next page"
                  disabled={page >= totalPages}
                  onClick={() => changePage(page + 1)}
                >
                  Next
                  <ChevronRight className="ml-1 h-3.5 w-3.5" aria-hidden="true" />
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card className="min-w-0">
            <CardHeader className="p-4">
              <CardTitle>Request detail</CardTitle>
              <CardDescription>Performance, volume, identity, and status for the selected request.</CardDescription>
            </CardHeader>
            <CardContent className="p-4 pt-0">
              <RequestEvidenceEvent event={selectedEvent} label="Selected request" />
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}

function RequestListItem({
  event,
  number,
  selected,
  onSelect,
}: {
  event: UsageEvent
  number: number
  selected: boolean
  onSelect: () => void
}) {
  const { keyLabel } = getRequestEventLabels(event)

  return (
    <button
      type="button"
      aria-label={`Select request ${number}`}
      aria-pressed={selected}
      onClick={onSelect}
      className={cn(
        "grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-lg border p-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500",
        selected
          ? "border-terracotta-300 bg-terracotta-50 dark:border-terracotta-800 dark:bg-terracotta-950/30"
          : "border-border hover:bg-muted/60",
      )}
    >
      <div className="min-w-0">
        <p className="truncate text-sm font-medium">{keyLabel}</p>
        <p className="mt-0.5 truncate text-xs text-muted-foreground">
          {event.model || "Unknown model"} · {formatDate(event.timestamp)}
        </p>
      </div>
      <div className="min-w-0 text-right">
        <p className="whitespace-nowrap text-sm font-medium">{formatOutputTPS(event.output_tps)}</p>
        <p className="mt-0.5 whitespace-nowrap text-xs text-muted-foreground">
          {formatCompact(event.tokens?.total_tokens ?? 0, 2)} · {event.failed ? "Failed" : "Success"}
        </p>
      </div>
    </button>
  )
}

function requestEventKey(event: UsageEvent) {
  return String(event.id ?? `${event.timestamp}:${event.source}:${event.model}`)
}
