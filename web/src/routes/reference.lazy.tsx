import { createLazyFileRoute } from "@tanstack/react-router"
import { Check, Pencil, Search, Trash2, X } from "lucide-react"
import { RateInput } from "@/components/reference/rate-input"
import { SummaryCard } from "@/components/reference/summary-card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { KEY_ALIAS_SCOPES } from "@/features/reference-data/model"
import { useReferenceDataWorkbench } from "@/features/reference-data/use-reference-data-workbench"
import { formatCompact, formatCost, formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"

export const Route = createLazyFileRoute("/reference")({
  component: ReferencePage,
})

export function ReferencePage() {
  const {
    query,
    setQuery,
    keyAliasScope,
    selectKeyAliasScope,
    scopeDescription,
    apiKeyCount,
    accountCount,
    aliasedAPIKeys,
    aliasedAccounts,
    missingRates,
    apiKeysRead,
    accountsRead,
    pricingRead,
    aliasRead,
    filteredKeys,
    editingId,
    draftAlias,
    setDraftAlias,
    startEdit,
    cancelEdit,
    saveEdit,
    clearEdit,
    models,
    pricingMap,
    getDraft,
    updateDraft,
    saveRate,
    savingModel,
  } = useReferenceDataWorkbench()

  return (
    <div className="animate-slide-up mx-auto max-w-7xl space-y-6">
      <header>
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          Reference Data
        </p>
        <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight">
          Reference
        </h1>
      </header>

      <div className="grid gap-3 md:grid-cols-3">
        <SummaryCard label="API Keys" value={apiKeyCount} caption={aliasedAPIKeys === undefined ? undefined : `${aliasedAPIKeys} aliased`} loading={apiKeysRead.status === "loading"} error={apiKeysRead.status === "error"} refreshError={Boolean(apiKeysRead.refreshError)} onRetry={apiKeysRead.retry} />
        <SummaryCard label="Accounts" value={accountCount} caption={aliasedAccounts === undefined ? undefined : `${aliasedAccounts} aliased`} loading={accountsRead.status === "loading"} error={accountsRead.status === "error"} refreshError={Boolean(accountsRead.refreshError)} onRetry={accountsRead.retry} />
        <SummaryCard label="Missing Cost Rates" value={missingRates} caption={missingRates === undefined ? undefined : "Models without configured rates"} loading={pricingRead.status === "loading"} error={pricingRead.status === "error"} refreshError={Boolean(pricingRead.refreshError)} onRetry={pricingRead.retry} tone={(missingRates ?? 0) > 0 ? "amber" : "green"} />
      </div>

      <Card>
        <CardHeader className="flex flex-col items-start justify-between gap-4 md:flex-row md:flex-wrap">
          <div>
            <CardTitle>Key Aliases</CardTitle>
            <CardDescription>{scopeDescription}</CardDescription>
          </div>
          <div className="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center md:justify-end">
            <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-border bg-card p-1">
              {KEY_ALIAS_SCOPES.map((item) => (
                <button
                  key={item.value}
                  onClick={() => {
                    selectKeyAliasScope(item.value)
                  }}
                  aria-label={`Key alias scope: ${item.label}`}
                  aria-pressed={keyAliasScope === item.value}
                  className={cn(
                    "shrink-0 rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
                    keyAliasScope === item.value
                      ? "bg-terracotta-500 text-white"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
            <div className="flex w-full items-center gap-2 rounded-lg border border-border bg-background px-3 py-2 md:w-auto">
              <Search className="h-4 w-4 text-muted-foreground" />
              <input
                name="key-alias-search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search alias or key..."
                className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground md:min-w-[200px]"
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {aliasRead.refreshError ? (
            <div className="mb-3 flex items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
              <span>Key alias refresh failed; showing the last complete result.</span>
              <Button type="button" size="sm" variant="outline" onClick={aliasRead.retry}>Retry</Button>
            </div>
          ) : null}
          <div className="space-y-2">
            {aliasRead.status === "loading" ? (
              <>
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </>
            ) : aliasRead.status === "error" ? (
              <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border p-6 text-center text-sm text-red-600">
                <span>Failed to load {keyAliasScope === "api-key" ? "API keys" : "accounts"}</span>
                <Button type="button" size="sm" variant="outline" onClick={aliasRead.retry}>Retry key aliases</Button>
              </div>
            ) : filteredKeys.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
                No keys found
              </div>
            ) : (
              filteredKeys.map((key) => {
                const editing = editingId === key.id
                const label = key.alias || key.displayName || key.name || key.identity
                return (
                  <div
                    key={key.id}
                    className="grid items-center gap-3 rounded-lg border border-border p-3 md:grid-cols-[1fr_120px_130px_100px]"
                  >
                    <div className="min-w-0">
                      {editing ? (
                        <input
                          name={`key-alias-${key.id}`}
                          value={draftAlias}
                          onChange={(event) => setDraftAlias(event.target.value)}
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
                            <span className="sr-only">Save alias for {label}</span>
                            <Check className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => cancelEdit()}>
                            <span className="sr-only">Cancel alias edit for {label}</span>
                            <X className="h-4 w-4" />
                          </Button>
                        </>
                      ) : (
                        <>
                          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => startEdit(key)}>
                            <span className="sr-only">Edit alias for {label}</span>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            disabled={!key.alias}
                            onClick={() => void clearEdit(key)}
                          >
                            <span className="sr-only">Clear alias for {label}</span>
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

      <Card>
        <CardHeader>
          <CardTitle>Cost Rates</CardTitle>
          <CardDescription>Model unit rates used by Cost calculations</CardDescription>
        </CardHeader>
        <CardContent>
          {pricingRead.refreshError ? (
            <div className="mb-3 flex items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
              <span>Cost rate refresh failed; showing the last complete result.</span>
              <Button type="button" size="sm" variant="outline" onClick={pricingRead.retry}>Retry</Button>
            </div>
          ) : null}
          <div className="space-y-2">
            {pricingRead.status === "loading" ? (
              <>
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </>
            ) : pricingRead.status === "error" ? (
              <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border p-6 text-center text-sm text-red-600">
                <span>Failed to load cost rates</span>
                <Button type="button" size="sm" variant="outline" onClick={pricingRead.retry}>Retry cost rates</Button>
              </div>
            ) : models.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
                No models available for cost rates
              </div>
            ) : (
              models.map((model) => {
                const draft = getDraft(model)
                const configured = pricingMap.has(model)
                return (
                  <div
                    key={model}
                    className="grid items-end gap-3 rounded-lg border border-border p-3 md:grid-cols-[1fr_120px_120px_120px_auto]"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{model}</p>
                      <Badge variant={configured ? "green" : "amber"} className="mt-1 text-[10px]">
                        {configured ? "Configured" : "Missing rate"}
                      </Badge>
                    </div>
                    <RateInput label="Prompt" value={draft.prompt} onChange={(value) => updateDraft(model, "prompt", value)} />
                    <RateInput label="Completion" value={draft.completion} onChange={(value) => updateDraft(model, "completion", value)} />
                    <RateInput label="Cache" value={draft.cache} onChange={(value) => updateDraft(model, "cache", value)} />
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-9"
                      disabled={savingModel === model}
                      onClick={() => void saveRate(model)}
                    >
                      Save
                    </Button>
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
