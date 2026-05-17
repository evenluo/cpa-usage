import { createLazyFileRoute } from "@tanstack/react-router"
import { useQueryClient, useMutation } from "@tanstack/react-query"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { useStatus } from "@/hooks/useStatus"
import { useAuth } from "@/hooks/useAuth"
import { apiFetch } from "@/lib/api"
import { formatDate } from "@/lib/format"
import { useToast } from "@/components/providers/toast-provider"
import { RefreshCw } from "lucide-react"

export const Route = createLazyFileRoute("/settings")({
  component: SettingsPage,
})

function SettingsPage() {
  const { data: status, isLoading } = useStatus()
  const { data: auth } = useAuth()
  const toast = useToast()
  const qc = useQueryClient()

  const syncMutation = useMutation({
    mutationFn: () => apiFetch("/sync", { method: "POST" }),
    onSuccess: () => {
      toast.success("Sync triggered")
      qc.invalidateQueries({ queryKey: ["status"] })
    },
    onError: (err: Error) => {
      toast.error(err.message || "Sync failed")
    },
  })

  return (
    <div className="animate-slide-up space-y-6">
      <header>
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          System Settings
        </p>
        <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight">
          Settings
        </h1>
      </header>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>Sync Status</CardTitle>
            <CardDescription>Ingestion and manual sync state</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-20 w-full" />
            ) : (
              <>
                <div className="flex items-center gap-2">
                  <RefreshCw className="h-4 w-4 text-muted-foreground" />
                  <p className="text-sm font-semibold">
                    {status?.last_status || "No sync status"}
                  </p>
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  {status?.last_run_at ? formatDate(status.last_run_at) : "Never run"}
                </p>
                {status?.last_error && (
                  <p className="mt-2 text-xs text-red-600">{status.last_error}</p>
                )}
                {status?.last_warning && (
                  <p className="mt-2 text-xs text-amber-700">{status.last_warning}</p>
                )}
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-4"
                  disabled={syncMutation.isPending || status?.sync_running}
                  onClick={() => syncMutation.mutate()}
                >
                  {syncMutation.isPending ? "Syncing..." : "Trigger Sync"}
                </Button>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Version</CardTitle>
            <CardDescription>Current deployment info</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-16 w-full" />
            ) : (
              <>
                <p className="text-sm font-semibold">{status?.version || "dev"}</p>
                <p className="mt-2 text-xs text-muted-foreground">
                  {status?.updateCheckEnabled
                    ? "Update check enabled"
                    : "Update check disabled"}
                </p>
                <Badge variant="outline" className="mt-3">
                  {status?.timezone || "Local timezone"}
                </Badge>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Auth</CardTitle>
            <CardDescription>Session state</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm font-semibold">
              {auth?.authenticated ? "Authenticated" : "Not authenticated"}
            </p>
            <p className="mt-2 text-xs text-muted-foreground">
              Dashboard password required
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
