import { cn } from "@/lib/utils"
import { type HTMLAttributes } from "react"

export interface BadgeProps extends HTMLAttributes<HTMLDivElement> {
  variant?:
    | "default"
    | "secondary"
    | "outline"
    | "terracotta"
    | "green"
    | "amber"
    | "red"
    | "blue"
}

export function Badge({
  className,
  variant = "default",
  ...props
}: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors",
        {
          "border-transparent bg-terracotta-500 text-white":
            variant === "default",
          "border-transparent bg-secondary text-secondary-foreground":
            variant === "secondary",
          "border-border bg-background": variant === "outline",
          "border-transparent bg-terracotta-500/10 text-terracotta-700":
            variant === "terracotta",
          "border-transparent bg-emerald-500/10 text-emerald-700":
            variant === "green",
          "border-transparent bg-amber-500/10 text-amber-700":
            variant === "amber",
          "border-transparent bg-red-500/10 text-red-700": variant === "red",
          "border-transparent bg-blue-500/10 text-blue-700": variant === "blue",
        },
        className
      )}
      {...props}
    />
  )
}
