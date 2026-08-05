import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Sparkline } from "@/components/charts/sparkline"
import { useCountUp } from "@/hooks/useCountUp"

export interface KpiCardProps {
  label: string
  rawValue?: number
  formatter?: (n: number) => string
  caption?: string
  comparison?: string
  sparkline?: (number | null)[]
  valueDecimals?: number
  isLoading: boolean
  tone: "terracotta" | "blue" | "violet" | "green" | "amber"
}

const toneStyles = {
  terracotta: "text-terracotta-700 bg-terracotta-50 border-terracotta-200",
  blue: "text-blue-700 bg-blue-50 border-blue-200",
  violet: "text-violet-700 bg-violet-50 border-violet-200",
  green: "text-emerald-700 bg-emerald-50 border-emerald-200",
  amber: "text-amber-700 bg-amber-50 border-amber-200",
}

export function KpiCard({ label, rawValue, formatter, caption, comparison, sparkline, valueDecimals = 0, isLoading, tone }: KpiCardProps) {
  const animated = useCountUp(rawValue ?? 0, {
    duration: 900,
    decimals: valueDecimals,
    enabled: rawValue !== undefined,
  })
  const display = rawValue !== undefined && formatter ? formatter(animated) : "—"

  return (
    <Card>
      <CardContent className="p-5">
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-8 w-28" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : (
          <>
            <div
              className={`mb-3 inline-flex rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider ${toneStyles[tone]}`}
            >
              {label}
            </div>
            <p className="font-serif text-2xl font-semibold tracking-tight" data-testid={`kpi-value-${label.toLowerCase()}`}>{display}</p>
            {comparison && (
              <p className="mt-1 text-xs font-medium text-muted-foreground">{comparison}</p>
            )}
            <div className="mt-3 h-10">
              {sparkline && <Sparkline data={sparkline} />}
            </div>
            {caption && (
              <p className="mt-2 text-xs text-muted-foreground">{caption}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
