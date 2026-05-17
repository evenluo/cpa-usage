import { createLazyFileRoute } from "@tanstack/react-router"
import { useState, useMemo } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { useKeys, useUpdateAlias, useDeleteAlias } from "@/hooks/useKeys"
import { useToast } from "@/components/providers/toast-provider"
import { formatCompact, formatCost, formatDate } from "@/lib/format"
import { Search, Check, X, Pencil, Trash2 } from "lucide-react"

export const Route = createLazyFileRoute("/keys")({
  component: KeysPage,
})

function KeysPage() {
  const { data: keys, isLoading } = useKeys()
  const updateAlias = useUpdateAlias()
  const deleteAlias = useDeleteAlias()
  const toast = useToast()
  const [query, setQuery] = useState("")
  const [editingId, setEditingId] = useState<number | null>(null)
  const [draftAlias, setDraftAlias] = useState("")

  const filtered = useMemo(() => {
    if (!keys) return []
    const q = query.trim().toLowerCase()
    if (!q) return keys
    return keys.filter((k) =>
      [k.alias, k.displayName, k.name, k.identity, k.provider, k.type, k.auth_type_name]
        .some((v) => v?.toLowerCase().includes(q))
    )
  }, [keys, query])

  function startEdit(key: typeof filtered[0]) {
    setEditingId(key.id)
    setDraftAlias(key.alias)
  }

  async function saveEdit(key: typeof filtered[0]) {
    try {
      await updateAlias.mutateAsync({ id: key.id, alias: draftAlias })
      setEditingId(null)
      toast.success("Alias saved")
    } catch {
      toast.error("Failed to save alias")
    }
  }

  async function clearEdit(key: typeof filtered[0]) {
    try {
      await deleteAlias.mutateAsync(key.id)
      setEditingId(null)
      toast.success("Alias cleared")
    } catch {
      toast.error("Failed to clear alias")
    }
  }

  return (
    <div className="animate-slide-up space-y-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Key Management</p>
          <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight">Keys</h1>
        </div>
        <Badge variant="terracotta">{keys?.length ?? 0} keys</Badge>
      </header>

      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <CardTitle>Key Alias Directory</CardTitle>
            <CardDescription>Aliases are stored locally and do not change CPA source data</CardDescription>
          </div>
          <div className="flex items-center gap-2 rounded-lg border border-border bg-background px-3 py-2">
            <Search className="h-4 w-4 text-muted-foreground" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search alias or key..."
              className="min-w-[200px] bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {isLoading ? (
              <>
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </>
            ) : filtered.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
                No keys found
              </div>
            ) : (
              filtered.map((key) => {
                const editing = editingId === key.id
                const label = key.alias || key.displayName || key.name || key.identity
                return (
                  <div
                    key={key.id}
                    className="grid items-center gap-3 rounded-lg border border-border p-3 sm:grid-cols-[1fr_120px_130px_100px]"
                  >
                    <div className="min-w-0">
                      {editing ? (
                        <input
                          value={draftAlias}
                          onChange={(e) => setDraftAlias(e.target.value)}
                          className="h-9 w-full rounded-md border border-border bg-background px-3 text-sm font-medium outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
                          maxLength={80}
                          autoFocus
                        />
                      ) : (
                        <p className="truncate text-sm font-medium">{label}</p>
                      )}
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">{key.identity}</p>
                      <div className="mt-1.5 flex flex-wrap gap-1">
                        <Badge variant="outline" className="text-[10px]">{key.provider}</Badge>
                        <Badge variant="outline" className="text-[10px]">{key.type}</Badge>
                        <Badge variant="outline" className="text-[10px]">{key.auth_type_name}</Badge>
                      </div>
                    </div>

                    <div>
                      <p className="text-[10px] font-medium uppercase text-muted-foreground">Last used</p>
                      <p className="mt-0.5 text-sm font-medium">{formatDate(key.last_used_at)}</p>
                    </div>

                    <div>
                      <p className="text-[10px] font-medium uppercase text-muted-foreground">Usage</p>
                      <p className="mt-0.5 text-sm font-medium">{formatCompact(key.total_tokens, 2)} tokens</p>
                      <p className="text-xs text-muted-foreground">
                        {key.cost_available ? formatCost(key.total_cost) : "Cost unavailable"}
                      </p>
                    </div>

                    <div className="flex justify-end gap-1">
                      {editing ? (
                        <>
                          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => void saveEdit(key)}>
                            <Check className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setEditingId(null)}>
                            <X className="h-4 w-4" />
                          </Button>
                        </>
                      ) : (
                        <>
                          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => startEdit(key)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            disabled={!key.alias}
                            onClick={() => void clearEdit(key)}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
