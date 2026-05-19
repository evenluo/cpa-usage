import { Link, useRouter } from "@tanstack/react-router"
import {
  BarChart3,
  Activity,
  Database,
} from "lucide-react"
import { cn } from "@/lib/utils"

const navItems = [
  { label: "Intelligence", href: "/", icon: BarChart3 },
  { label: "Reference", href: "/reference", icon: Database },
  { label: "Operations", href: "/operations", icon: Activity },
]

export function MobileNav() {
  const router = useRouter()
  const currentPath = router.state.location.pathname

  return (
    <nav aria-label="Mobile navigation" className="fixed inset-x-0 bottom-0 z-50 translate-y-0 border-t border-border bg-card/95 pb-[env(safe-area-inset-bottom)] shadow-[0_-8px_24px_rgba(0,0,0,0.08)] backdrop-blur will-change-transform supports-[backdrop-filter]:bg-card/80 md:hidden">
      <div className="flex h-16 items-center justify-around px-2">
        {navItems.map((item) => {
          const isActive = currentPath === item.href
          return (
            <Link
              key={item.href}
              to={item.href}
              aria-label={item.label}
              className={cn(
                "flex flex-col items-center justify-center gap-0.5 rounded-lg px-3 py-1.5 transition-colors",
                isActive
                  ? "text-terracotta-600"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              <item.icon className="h-5 w-5" />
              <span className="text-[10px] font-medium">{item.label}</span>
            </Link>
          )
        })}
      </div>
    </nav>
  )
}
