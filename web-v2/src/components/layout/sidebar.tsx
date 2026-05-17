import { Link, useRouter } from "@tanstack/react-router"
import {
  BarChart3,
  Activity,
  Database,
  Sun,
  Moon,
} from "lucide-react"
import { useTheme } from "@/components/providers/theme-provider"
import { cn } from "@/lib/utils"
import { useState } from "react"

const navItems = [
  { label: "Intelligence", href: "/", icon: BarChart3 },
  { label: "Reference", href: "/reference", icon: Database },
  { label: "Operations", href: "/operations", icon: Activity },
]

export function Sidebar() {
  const { theme, setTheme, resolvedTheme } = useTheme()
  const router = useRouter()
  const currentPath = router.state.location.pathname
  const [hovered, setHovered] = useState<string | null>(null)

  return (
    <aside className="fixed left-0 top-0 z-40 flex h-screen w-16 flex-col items-center border-r border-border bg-card py-4 transition-all duration-300">
      {/* Logo */}
      <div className="mb-6 flex h-9 w-9 items-center justify-center rounded-lg bg-terracotta-500 text-white">
        <BarChart3 className="h-5 w-5" />
      </div>

      {/* Nav Icons */}
      <nav className="flex flex-1 flex-col items-center gap-1">
        {navItems.map((item) => {
          const isActive = currentPath === item.href
          return (
            <div
              key={item.href}
              className="relative"
              onMouseEnter={() => setHovered(item.label)}
              onMouseLeave={() => setHovered(null)}
            >
              <Link
                to={item.href}
                className={cn(
                  "flex h-10 w-10 items-center justify-center rounded-lg transition-colors duration-200",
                  isActive
                    ? "bg-terracotta-500/10 text-terracotta-600"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                )}
              >
                <item.icon className="h-[18px] w-[18px]" />
              </Link>
              {/* Tooltip */}
              {hovered === item.label && (
                <div className="absolute left-full ml-2 top-1/2 -translate-y-1/2 whitespace-nowrap rounded-md bg-foreground px-2.5 py-1 text-xs font-medium text-background shadow-lg animate-fade-in">
                  {item.label}
                </div>
              )}
              {/* Active indicator dot */}
              {isActive && (
                <div className="absolute -right-[7px] top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full bg-terracotta-500" />
              )}
            </div>
          )
        })}
      </nav>

      {/* Theme Toggle */}
      <div className="flex flex-col items-center gap-1">
        <button
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          className="flex h-10 w-10 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          title={`Current: ${resolvedTheme}`}
        >
          {resolvedTheme === "dark" ? (
            <Moon className="h-[18px] w-[18px]" />
          ) : (
            <Sun className="h-[18px] w-[18px]" />
          )}
        </button>
      </div>
    </aside>
  )
}
