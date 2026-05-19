import { useEffect, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useEvents } from "@/hooks/useEvents"
import { formatCompact, formatDate } from "@/lib/format"
import type { UsageEvent } from "@/types/api"

export function RequestEvidence({ provider }: { provider: string }) {
  const { data, isLoading, error } = useEvents("24h", 10, provider)
  const events = data?.events.slice(0, 5) ?? []
  const eventFingerprint = events.map(getEventIdentity).join("|")
  const rotationKey = `${provider}:${eventFingerprint}`
  const [rotation, setRotation] = useState({ key: rotationKey, index: 0 })
  const activeIndex = rotation.key === rotationKey ? rotation.index : 0
  const activeEvent = events[activeIndex % events.length]
  const queuedEvents = events.length === 0
    ? []
    : events.map((_, index) => events[(activeIndex + index) % events.length])

  useEffect(() => {
    if (events.length <= 1) return

    const interval = window.setInterval(() => {
      if (document.visibilityState === "hidden") return
      setRotation((current) => ({
        key: rotationKey,
        index: current.key === rotationKey ? (current.index + 1) % events.length : 1 % events.length,
      }))
    }, 5_500)

    return () => window.clearInterval(interval)
  }, [events.length, rotationKey])

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
          <div className="h-full space-y-2 overflow-y-auto pr-1">
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
          <div className="flex h-full min-w-0 flex-col gap-2">
            {activeEvent && <EvidenceSpotlight event={activeEvent} key={getEventIdentity(activeEvent)} />}
            <div className="grid grid-cols-5 gap-1">
              {events.map((event, index) => (
                <button
                  key={getEventIdentity(event)}
                  type="button"
                  aria-label={`Show request evidence ${index + 1}`}
                  aria-pressed={activeIndex % events.length === index}
                  onClick={() => setRotation({ key: rotationKey, index })}
                  className={
                    activeIndex % events.length === index
                      ? "h-1.5 rounded-full bg-terracotta-500 transition-colors"
                      : "h-1.5 rounded-full bg-muted transition-colors hover:bg-muted-foreground/30"
                  }
                />
              ))}
            </div>
            <div className="min-h-0 min-w-0 flex-1 space-y-1.5 overflow-hidden">
              {queuedEvents.slice(1, 4).map((event) => (
                <EvidenceQueueRow event={event} key={getEventIdentity(event)} />
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function EvidenceSpotlight({ event }: { event: UsageEvent }) {
  const { keyLabel, keyTrace } = getEventLabels(event)

  return (
    <div className="animate-slide-up min-w-0 rounded-lg border border-terracotta-200 bg-terracotta-50/70 p-3 dark:border-terracotta-900/60 dark:bg-terracotta-950/20">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold">{keyLabel}</p>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">{keyTrace}</p>
        </div>
        <Badge variant={event.failed ? "amber" : "green"} className="shrink-0 text-[10px]">
          {event.failed ? "Failed" : "Success"}
        </Badge>
      </div>
      <div className="mt-3 grid min-w-0 grid-cols-2 gap-2 text-xs">
        <div className="min-w-0">
          <p className="truncate text-muted-foreground">Model</p>
          <p className="truncate font-medium">{event.model || "Unknown model"}</p>
        </div>
        <div className="min-w-0 text-right">
          <p className="truncate text-muted-foreground">Tokens</p>
          <p className="truncate font-medium">
            {formatCompact(event.tokens?.total_tokens ?? 0, 2)}
            {event.latency_ms > 0 ? ` · ${event.latency_ms}ms` : ""}
          </p>
        </div>
      </div>
      <p className="mt-2 truncate text-xs text-muted-foreground">{formatDate(event.timestamp)}</p>
    </div>
  )
}

function EvidenceQueueRow({ event }: { event: UsageEvent }) {
  const { keyLabel } = getEventLabels(event)

  return (
    <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 rounded-md border border-border bg-background/45 px-2.5 py-1.5 text-xs">
      <div className="min-w-0">
        <p className="truncate font-medium">{keyLabel}</p>
        <p className="truncate text-muted-foreground">{event.model || "Unknown model"}</p>
      </div>
      <div className="shrink-0 text-right text-muted-foreground">
        <p>{formatCompact(event.tokens?.total_tokens ?? 0, 1)}</p>
        <p>{event.failed ? "Failed" : "Success"}</p>
      </div>
    </div>
  )
}

function getEventLabels(event: UsageEvent) {
  const keyLabel = event.api_key_alias || event.api_key_display || event.source || event.auth_index || "No key trace"
  const keyTrace = [
    event.api_key_alias ? event.api_key_display : "",
    event.source || event.auth_index || "",
  ]
    .filter(Boolean)
    .join(" · ")

  return { keyLabel, keyTrace }
}

function getEventIdentity(event: UsageEvent) {
  return `${event.id ?? event.timestamp}-${event.auth_index ?? event.source}-${event.model}`
}
