import type { LucideIcon } from "lucide-react"

export function SectionDivider({ icon: Icon, label }: { icon: LucideIcon; label: string }) {
  return (
    <div className="flex items-center gap-3">
      <div className="h-px flex-1 bg-border" />
      <span className="flex items-center gap-1 text-xs text-muted-foreground">
        <Icon className="h-3 w-3" />
        {label}
      </span>
      <div className="h-px flex-1 bg-border" />
    </div>
  )
}
