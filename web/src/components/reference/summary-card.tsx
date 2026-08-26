import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

export interface SummaryCardProps {
  label: string
  value: number | string
  caption: string
  loading: boolean
  tone?: "terracotta" | "green" | "amber"
}

export function SummaryCard({
  label,
  value,
  caption,
  loading,
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
        ) : (
          <>
            <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{label}</p>
            <p className={`mt-2 font-serif text-2xl font-semibold tracking-tight ${toneClass}`}>{value}</p>
            <p className="mt-1 text-xs text-muted-foreground">{caption}</p>
          </>
        )}
      </CardContent>
    </Card>
  )
}
