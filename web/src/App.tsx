import { type ReactNode } from "react"
import { ThemeProvider } from "@/components/providers/theme-provider"
import { ToastProvider } from "@/components/providers/toast-provider"
import { Sidebar } from "@/components/layout/sidebar"
import { MobileNav } from "@/components/layout/mobile-nav"

export default function App({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <ToastProvider>
        <div className="min-h-dvh bg-background">
          <Sidebar />
          <main className="ml-0 min-h-dvh min-w-0 overflow-x-clip p-4 pb-[calc(5rem+env(safe-area-inset-bottom))] md:ml-16 md:p-6 md:pb-6 lg:p-8">
            {children}
          </main>
          <MobileNav />
        </div>
      </ToastProvider>
    </ThemeProvider>
  )
}
