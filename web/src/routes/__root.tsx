import { createRootRoute, Outlet, useLocation } from "@tanstack/react-router"
import App from "@/App"
import { ProtectedSessionShell } from "@/components/layout/protected-session-shell"
import { useAuth } from "@/hooks/useAuth"
import { appBasePath } from "@/lib/api"
import { useCallback } from "react"

export const Route = createRootRoute({
  component: RootComponent,
})

function RootComponent() {
  const { data: auth, isLoading, error, isFetching, refetch } = useAuth()
  const location = useLocation()
  const isLoginPage = location.pathname === "/login"
  const redirectToLogin = useCallback(() => {
    window.location.assign(`${appBasePath()}/login`)
  }, [])

  if (isLoginPage) {
    return (
      <App>
        <Outlet />
      </App>
    )
  }

  return (
    <App>
      <ProtectedSessionShell
        session={auth}
        isLoading={isLoading}
        error={error}
        isRetrying={isFetching}
        onRetry={() => {
          void refetch()
        }}
        onUnauthenticated={redirectToLogin}
      >
        <Outlet />
      </ProtectedSessionShell>
    </App>
  )
}
