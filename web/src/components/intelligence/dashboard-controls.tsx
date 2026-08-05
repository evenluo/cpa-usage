import { CalendarRange, Clock, Filter } from "lucide-react"
import { cn } from "@/lib/utils"
import { formatCompact } from "@/lib/format"
import { TIME_RANGES } from "@/features/usage-intelligence/view-model"
import type { ProviderOption, TimeGranularity, TimeRange } from "@/types/api"

interface DashboardControlsProps {
  range: TimeRange
  onSelectRange: (range: TimeRange) => void
  onSelectGranularity: (granularity: TimeGranularity | null) => void
  effectiveGranularity: TimeGranularity
  provider: string
  onSelectProvider: (provider: string) => void
  providerOptions: ProviderOption[]
}

export function DashboardControls({
  range,
  onSelectRange,
  onSelectGranularity,
  effectiveGranularity,
  provider,
  onSelectProvider,
  providerOptions,
}: DashboardControlsProps) {
  return (
    <>
      {/* Header */}
      <header>
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Usage Intelligence
          </p>
          <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight text-foreground">
            Dashboard
          </h1>
        </div>
      </header>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        {/* Provider Filter */}
        {providerOptions.length > 0 ? (
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <Filter className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <button
              onClick={() => onSelectProvider("")}
              className={cn(
                "rounded-full px-3 py-1 text-xs font-medium transition-colors",
                provider === ""
                  ? "bg-foreground text-background"
                  : "bg-muted text-muted-foreground hover:bg-muted/80"
              )}
            >
              All
            </button>
            {providerOptions.map((opt) => (
              <button
                key={opt.provider}
                onClick={() => onSelectProvider(opt.provider)}
                className={cn(
                  "rounded-full px-3 py-1 text-xs font-medium transition-colors",
                  provider === opt.provider
                    ? "bg-foreground text-background"
                    : "bg-muted text-muted-foreground hover:bg-muted/80"
                )}
              >
                {opt.provider}
                <span className="ml-1 text-[10px] opacity-60">
                  {formatCompact(opt.request_count, 0)}
                </span>
              </button>
            ))}
          </div>
        ) : (
          <div />
        )}

        <div className="flex w-full flex-col items-end gap-1 sm:w-auto">
          <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
            {/* Time Range */}
            <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
              {TIME_RANGES.map((tr) => (
                <button
                  key={tr.value}
                  onClick={() => onSelectRange(tr.value)}
                  className={cn(
                    "shrink-0 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                    range === tr.value
                      ? "bg-terracotta-500 text-white"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  )}
                >
                  {tr.label}
                </button>
              ))}
            </div>

            {/* Granularity Toggle */}
            <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
              <button
                onClick={() => onSelectGranularity("hour")}
                className={cn(
                  "flex shrink-0 items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  effectiveGranularity === "hour"
                    ? "bg-terracotta-500 text-white"
                    : "text-muted-foreground hover:bg-muted"
                )}
              >
                <Clock className="h-3 w-3" />
                Hour
              </button>
              <button
                onClick={() => onSelectGranularity("day")}
                className={cn(
                  "flex shrink-0 items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  effectiveGranularity === "day"
                    ? "bg-terracotta-500 text-white"
                    : "text-muted-foreground hover:bg-muted"
                )}
              >
                <CalendarRange className="h-3 w-3" />
                Day
              </button>
            </div>
          </div>

          {/* Scope indicator */}
          <span className="flex items-center justify-end gap-1 text-right text-[10px] text-muted-foreground/60">
            <Clock className="h-3 w-3" />
            Applies to KPIs, Trend & Leaderboard
          </span>
        </div>
      </div>
    </>
  )
}
