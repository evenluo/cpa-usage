import { Link, useRouter } from "@tanstack/react-router"
import {
  BarChart3,
  KeyRound,
  Settings,
  TableProperties,
  WalletCards,
} from "lucide-react"
import { cn } from "@/lib/utils"

const navItems = [
  { label: "Analytics", href: "/", icon: BarChart3 },
  { label: "Keys", href: "/keys", icon: KeyRound },
  { label: "Events", href: "/events", icon: TableProperties },
  { label: "Pricing", href: "/pricing", icon: WalletCards },
  { label: "Settings", href: "/settings", icon: Settings },
]

export function MobileNav() {
  const router = useRouter()
  const currentPath = router.state.location.pathname

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 border-t border-border bg-card/95 backdrop-blur supports-[backdrop-filter]:bg-card/80 md:hidden">
      <div className="flex h-16 items-center justify-around px-2">
        {navItems.map((item) => {
          const isActive = currentPath === item.href
          return (
            <Link
              key={item.href}
              to={item.href}
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
