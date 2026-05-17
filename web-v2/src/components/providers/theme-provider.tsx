import { createContext, useContext, useEffect, useState, type ReactNode } from "react"

type Theme = "light" | "dark" | "system"

interface ThemeContextValue {
  theme: Theme
  resolvedTheme: "light" | "dark"
  setTheme: (theme: Theme) => void
}

const ThemeContext = createContext<ThemeContextValue | undefined>(undefined)

function getSystemTheme(): "light" | "dark" {
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(() => {
    const stored = localStorage.getItem("cpa-theme") as Theme | null
    return stored ?? "system"
  })
  const [resolvedTheme, setResolvedTheme] = useState<"light" | "dark">(getSystemTheme())

  useEffect(() => {
    const root = document.documentElement
    const resolved = theme === "system" ? getSystemTheme() : theme
    setResolvedTheme(resolved)
    root.classList.remove("light", "dark")
    root.classList.add(resolved)
  }, [theme])

  useEffect(() => {
    const listener = (e: MediaQueryListEvent) => {
      if (theme === "system") {
        const resolved = e.matches ? "dark" : "light"
        setResolvedTheme(resolved)
        document.documentElement.classList.remove("light", "dark")
        document.documentElement.classList.add(resolved)
      }
    }
    const mql = window.matchMedia("(prefers-color-scheme: dark)")
    mql.addEventListener("change", listener)
    return () => mql.removeEventListener("change", listener)
  }, [theme])

  const setTheme = (newTheme: Theme) => {
    localStorage.setItem("cpa-theme", newTheme)
    setThemeState(newTheme)
  }

  return (
    <ThemeContext.Provider value={{ theme, resolvedTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider")
  return ctx
}
