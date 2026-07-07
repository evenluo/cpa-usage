import { createLazyFileRoute, useNavigate } from "@tanstack/react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { AlertCircle, CheckCircle2, KeyRound, RefreshCw, Server } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useToast } from "@/components/providers/toast-provider"
import { useAuth } from "@/hooks/useAuth"
import { useStatus } from "@/hooks/useStatus"
import { apiFetch } from "@/lib/api"
import { formatDate } from "@/lib/format"

export const Route = createLazyFileRoute("/operations")({
  component: OperationsPage,
})

function syncStatusLabel(status?: string): string {
  switch (status) {
    case "empty":
      return "No new usage events"
    case "completed":
      return "Completed"
    case "completed_with_warnings":
      return "Completed with warnings"
    case "failed":
      return "Failed"
    case "":
    case undefined:
      return "No sync status"
    default:
      return status.replace(/_/g, " ")
  }
}

function syncStatusDescription(status?: string): string {
  if (status === "empty") {
    return "Redis queue was empty at the last manual sync"
  }
  return "Last sync result"
}

function OperationsPage() {
  const { data: status, isLoading } = useStatus()
  const { data: auth } = useAuth()
  const toast = useToast()
  const qc = useQueryClient()
  const navigate = useNavigate()

  const syncMutation = useMutation({
    mutationFn: () => apiFetch("/sync", { method: "POST" }),
    onSuccess: () => {
      toast.success("Sync triggered")
      qc.invalidateQueries({ queryKey: ["analytics"] })
      qc.invalidateQueries({ queryKey: ["usage", "overview"] })
      qc.invalidateQueries({ queryKey: ["usage", "request-health"] })
      qc.invalidateQueries({ queryKey: ["events"] })
      qc.invalidateQueries({ queryKey: ["keys", "identities"] })
      qc.invalidateQueries({ queryKey: ["pricing"] })
    },
    onError: (err: Error) => {
      toast.error(err.message || "Sync failed")
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["status"] })
    },
  })

  const logoutMutation = useMutation({
    mutationFn: () => apiFetch("/auth/logout", { method: "POST" }),
    onSuccess: () => {
      toast.success("Logged out")
      qc.setQueryData(["auth", "session"], { authenticated: false })
      qc.invalidateQueries({ queryKey: ["auth", "session"] })
      navigate({ to: "/login" })
    },
    onError: (err: Error) => {
      toast.error(err.message || "Logout failed")
    },
  })

  return (
    <div className="animate-slide-up mx-auto max-w-7xl space-y-6">
      <header>
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          Operations Console
        </p>
        <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight">
          Operations
        </h1>
      </header>

      <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
        <Card>
          <CardHeader className="flex flex-row items-start justify-between gap-4">
            <div>
              <CardTitle>Operational Status</CardTitle>
              <CardDescription>Ingestion state and manual sync control</CardDescription>
            </div>
            <Badge variant={status?.sync_running ? "amber" : "green"}>
              {status?.sync_running ? "Running" : "Idle"}
            </Badge>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-36 w-full" />
            ) : (
              <div className="space-y-5">
                <div className="flex items-start gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-terracotta-500/10 text-terracotta-700">
                    <RefreshCw className="h-5 w-5" />
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm font-semibold">
                      {syncStatusLabel(status?.last_status)}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {status?.last_run_at ? `${formatDate(status.last_run_at)} · ${syncStatusDescription(status.last_status)}` : "Never run"}
                    </p>
                  </div>
                </div>

                {(status?.last_error || status?.last_warning) && (
                  <div className="space-y-2">
                    {status.last_error && (
                      <p className="rounded-lg border border-red-500/20 bg-red-500/5 p-3 text-xs text-red-600">
                        {status.last_error}
                      </p>
                    )}
                    {status.last_warning && (
                      <p className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 text-xs text-amber-700">
                        {status.last_warning}
                      </p>
                    )}
                  </div>
                )}

                <Button
                  variant="outline"
                  disabled={syncMutation.isPending || status?.sync_running}
                  onClick={() => syncMutation.mutate()}
                >
                  {syncMutation.isPending ? "Syncing..." : "Trigger Sync"}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Server className="h-4 w-4 text-muted-foreground" />
                Runtime
              </CardTitle>
              <CardDescription>Deployment and local runtime state</CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <Skeleton className="h-20 w-full" />
              ) : (
                <>
                  <p className="text-sm font-semibold">{status?.version || "dev"}</p>
                  <Badge variant="outline" className="mt-3">
                    {status?.timezone || "Local timezone"}
                  </Badge>
                </>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <KeyRound className="h-4 w-4 text-muted-foreground" />
                Access
              </CardTitle>
              <CardDescription>Dashboard session state</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-2">
                {auth?.authenticated ? (
                  <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                ) : (
                  <AlertCircle className="h-4 w-4 text-amber-600" />
                )}
                <p className="text-sm font-semibold">
                  {auth?.authenticated ? "Authenticated" : "Not authenticated"}
                </p>
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                Dashboard password required
              </p>
              <Button
                variant="outline"
                size="sm"
                className="mt-4"
                disabled={!auth?.authenticated || logoutMutation.isPending}
                onClick={() => logoutMutation.mutate()}
              >
                {logoutMutation.isPending ? "Logging out..." : "Log out"}
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
