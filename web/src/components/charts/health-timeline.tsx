import type { TrendPoint } from "@/types/api"
import { cellColor, groupByDate } from "@/features/usage-intelligence/health-timeline-model"
import { Fragment } from "react"

interface HealthTimelineProps {
  data: TrendPoint[]
  granularity?: "hour" | "day"
}

export function HealthTimeline({ data, granularity }: HealthTimelineProps) {
  if (data.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        No health data
      </div>
    )
  }

  const groups = groupByDate(data)
  const dates = Array.from(groups.keys()).sort()

  if (dates.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        No health data
      </div>
    )
  }

  // Explicit granularity instead of guessing from data distribution
  const isHourly = granularity === "hour"

  // For hourly: show 24 columns (hours), rows = days
  // For daily: show 1 column per day, single row
  const hours = isHourly ? Array.from({ length: 24 }, (_, h) => h) : []

  // Calculate overall stats
  const totalRequests = data.reduce((sum, p) => sum + p.request_count, 0)
  const totalFailures = data.reduce((sum, p) => sum + p.failure_count, 0)
  const overallRate = totalRequests > 0 ? ((totalRequests - totalFailures) / totalRequests) * 100 : 0

  return (
    <div className="space-y-3">
      {/* Overall stats */}
      <div className="flex items-center justify-between text-xs">
        <div className="flex items-center gap-3">
          <span className="text-muted-foreground">{totalRequests.toLocaleString("en")} requests</span>
          {totalFailures > 0 && (
            <span className="text-red-400">{totalFailures} failed</span>
          )}
        </div>
        <span className="font-medium text-emerald-500">{overallRate.toFixed(1)}%</span>
      </div>

      {/* Grid */}
      <div className="overflow-x-auto pb-1">
        {isHourly ? (
          <div
            className="grid gap-[3px]"
            style={{
              gridTemplateColumns: `60px repeat(24, minmax(10px, 1fr))`,
              minWidth: "fit-content",
            }}
          >
            {/* Header row with hour labels */}
            <div />
            {hours.map((h) => (
              <div
                key={h}
                className="text-center text-[9px] font-medium text-muted-foreground/50"
              >
                {h % 6 === 0 ? h : ""}
              </div>
            ))}

            {/* Data rows */}
            {dates.map((date) => {
              const blocks = groups.get(date)!
              const blockMap = new Map(blocks.map((b) => [b.hour, b]))

              return (
                <Fragment key={date}>
                  <div className="flex h-5 items-center truncate pr-2 text-[10px] font-medium text-muted-foreground">
                    {blocks[0]?.label}
                  </div>
                  {hours.map((h) => {
                    const block = blockMap.get(h)
                    const hasData = block !== undefined
                    const hasFailure = block ? block.failure > 0 : false
                    const rate = block ? block.rate : 0

                    return (
                      <div
                        key={`${date}-${h}`}
                        className={`h-5 rounded-[2px] transition-all duration-150 ${
                          hasData
                            ? `${cellColor(rate, hasFailure)} hover:scale-125 hover:rounded-sm hover:shadow-sm`
                            : "bg-muted/30"
                        }`}
                        title={
                          block
                            ? `${block.label} ${h}:00 — ${block.rate.toFixed(1)}% success (${block.failure} failures, ${block.success + block.failure} requests)`
                            : `${date} ${h}:00 — no data`
                        }
                      />
                    )
                  })}
                </Fragment>
              )
            })}
          </div>
        ) : (
          /* Daily granularity: single row of blocks */
          <div className="flex items-center gap-[3px]">
            {dates.map((date) => {
              const block = groups.get(date)![0]
              const hasFailure = block.failure > 0

              return (
                <div
                  key={date}
                  className={`h-6 flex-1 rounded-[3px] ${cellColor(block.rate, hasFailure)} transition-all hover:scale-110 hover:shadow-sm`}
                  title={`${block.label} — ${block.rate.toFixed(1)}% success (${block.failure} failures, ${block.success + block.failure} requests)`}
                />
              )
            })}
          </div>
        )}
      </div>

      {/* Legend */}
      <div className="flex items-center gap-3 text-[10px] text-muted-foreground/60">
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-emerald-500" />
          Perfect
        </div>
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-emerald-400" />
          ≥99%
        </div>
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-amber-400" />
          ≥95%
        </div>
        <div className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-[1px] bg-red-400" />
          &lt;95%
        </div>
      </div>
    </div>
  )
}
