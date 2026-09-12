import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"

interface DashboardCoreEmptyStateProps {
  refreshError?: unknown
  onRetry: () => void
}

export function DashboardCoreEmptyState({ refreshError, onRetry }: DashboardCoreEmptyStateProps) {
  return (
    <Card>
      <CardContent className="flex min-h-32 flex-col items-center justify-center gap-3 text-center">
        {refreshError ? (
          <>
            <p className="text-sm text-amber-700 dark:text-amber-300">
              Refresh failed. Last result was empty.
            </p>
            <Button type="button" size="sm" variant="outline" onClick={onRetry}>Retry</Button>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">No usage</p>
        )}
      </CardContent>
    </Card>
  )
}
