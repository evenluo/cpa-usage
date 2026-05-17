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

  useEffect(() => {
    if (!isLoading && !auth?.authenticated && !isLoginPage) {
      window.location.assign(`${window.__APP_BASE_PATH__ || ""}/login`)
    }
  }, [auth?.authenticated, isLoading, isLoginPage])

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
    return (
      <App>
        <div className="flex min-h-[80vh] items-center justify-center text-muted-foreground">
          Redirecting to sign in...
        </div>
      </App>
    )
  }

  return (
    <App>
      <Outlet />
    </App>
  )
}
