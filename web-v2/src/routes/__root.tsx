import { createRootRoute, Outlet, useLocation } from "@tanstack/react-router"
import App from "@/App"
import { useAuth } from "@/hooks/useAuth"
import { useEffect } from "react"

export const Route = createRootRoute({
  component: RootComponent,
})

function RootComponent() {
  const { data: auth, isLoading } = useAuth()
  const location = useLocation()
  const isLoginPage = location.pathname === "/login"

  if (isLoading) {
    return (
      <App>
        <div className="flex min-h-[80vh] items-center justify-center text-muted-foreground">
          Checking session...
        </div>
      </App>
    )
  }

  if (!auth?.authenticated && !isLoginPage) {
    // Redirect to login would be handled by router, for now just show login
    return (
      <App>
        <Outlet />
      </App>
    )
  }

  return (
    <App>
      <Outlet />
    </App>
  )
}
