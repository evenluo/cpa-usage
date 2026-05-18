import { cn } from "@/lib/utils"
import { type ButtonHTMLAttributes, forwardRef } from "react"

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "outline" | "ghost" | "subtle"
  size?: "default" | "sm" | "icon"
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(
          "inline-flex items-center justify-center rounded-lg font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500 disabled:pointer-events-none disabled:opacity-50",
          {
            "bg-terracotta-500 text-white hover:bg-terracotta-600":
              variant === "default",
            "border border-border bg-background hover:bg-muted":
              variant === "outline",
            "hover:bg-muted": variant === "ghost",
            "bg-terracotta-500/10 text-terracotta-700 hover:bg-terracotta-500/20":
              variant === "subtle",
          },
          {
            "h-10 px-4 py-2 text-sm": size === "default",
            "h-8 px-3 text-xs": size === "sm",
            "h-10 w-10": size === "icon",
          },
          className
        )}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"
