import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

export interface SummaryCardProps {
  label: string
  value?: number | string
  caption?: string
  loading: boolean
  error?: boolean
  refreshError?: boolean
  onRetry?: () => void
  tone?: "terracotta" | "green" | "amber"
}

export function SummaryCard({
  label,
  value,
  caption,
  loading,
  error = false,
  refreshError = false,
  onRetry,
  tone = "terracotta",
}: SummaryCardProps) {
  const toneClass = {
    terracotta: "text-terracotta-700",
    green: "text-emerald-700",
    amber: "text-amber-700",
  }[tone]

  return (
    <Card>
      <CardContent className="p-5">
        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-8 w-20" />
          </div>
        ) : error ? (
          <>
            <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{label}</p>
            <p className="mt-2 text-sm font-medium text-red-600">Unavailable</p>
            <div className="mt-1 flex items-center justify-between gap-2">
              <p className="text-xs text-muted-foreground">Read failed</p>
              {onRetry ? (
                <button type="button" className="text-xs font-medium text-terracotta-700 hover:underline dark:text-terracotta-300" onClick={onRetry}>Retry</button>
              ) : null}
            </div>
          </>
        ) : (
          <>
            <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{label}</p>
            <p className={`mt-2 font-serif text-2xl font-semibold tracking-tight ${toneClass}`}>{value ?? "—"}</p>
            <div className="mt-1 flex items-center justify-between gap-2">
              <p className="text-xs text-muted-foreground">{caption}</p>
              {refreshError && onRetry ? (
                <button type="button" className="text-xs font-medium text-terracotta-700 hover:underline dark:text-terracotta-300" onClick={onRetry}>Retry</button>
              ) : null}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}
