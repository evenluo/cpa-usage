import { useEffect, type ReactNode } from "react"
import { Button } from "@/components/ui/button"
import type { AuthSession } from "@/hooks/useAuth"

interface ProtectedSessionShellProps {
  session: AuthSession | undefined
  isLoading: boolean
  error: unknown
  isRetrying: boolean
  onRetry: () => void
  onUnauthenticated: () => void
  children: ReactNode
}

export function ProtectedSessionShell({
  session,
  isLoading,
  error,
  isRetrying,
  onRetry,
  onUnauthenticated,
  children,
}: ProtectedSessionShellProps) {
  useEffect(() => {
    if (!isLoading && !error && session?.authenticated === false) {
      onUnauthenticated()
    }
  }, [error, isLoading, onUnauthenticated, session?.authenticated])

  if (isLoading) {
    return (
      <div className="flex min-h-[80vh] items-center justify-center text-muted-foreground">
        Checking session...
      </div>
    )
  }

  if (error || session?.authenticated !== true) {
    if (!error && session?.authenticated === false) {
      return (
        <div className="flex min-h-[80vh] items-center justify-center text-muted-foreground">
          Redirecting to sign in...
        </div>
      )
    }
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center gap-3 text-center">
        <div>
          <p className="text-sm font-medium text-red-600">Unable to verify your session</p>
          <p className="mt-1 text-xs text-muted-foreground">Protected content remains unavailable until the session check succeeds.</p>
        </div>
        <Button type="button" size="sm" variant="outline" disabled={isRetrying} onClick={onRetry}>
          {isRetrying ? "Retrying..." : "Retry session check"}
        </Button>
      </div>
    )
  }

  return children
}
