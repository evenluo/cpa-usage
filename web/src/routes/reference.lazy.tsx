import { createLazyFileRoute } from "@tanstack/react-router"
import { useMemo, useState } from "react"
import { Check, Pencil, Search, Trash2, X } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useToast } from "@/components/providers/toast-provider"
import { useAPIKeys, useDeleteAPIKeyAlias, useDeleteAlias, useKeys, useUpdateAPIKeyAlias, useUpdateAlias } from "@/hooks/useKeys"
import { usePricing, useSavePricing } from "@/hooks/usePricing"
import {
  buildCostRateModels,
  buildCostRateSavePayload,
  buildPricingMap,
  canSaveKeyAlias,
  countAliasedRows,
  countMissingCostRates,
  filterKeyAliasRows,
  getCostRateDraft,
  normalizeAccountKeyRows,
  normalizeAPIKeyRows,
  validateCostRateDraft,
} from "@/features/reference-data/model"
import { formatCompact, formatCost, formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { CostRateDrafts, KeyAliasScope, ReferenceKeyRow } from "@/features/reference-data/model"

export const Route = createLazyFileRoute("/reference")({
  component: ReferencePage,
})

function ReferencePage() {
  const { data: keys, isLoading: isKeysLoading } = useKeys()
  const { data: apiKeys, isLoading: isAPIKeysLoading } = useAPIKeys()
  const { data: pricingData, isLoading: isPricingLoading } = usePricing()
  const updateAlias = useUpdateAlias()
  const updateAPIKeyAlias = useUpdateAPIKeyAlias()
  const deleteAlias = useDeleteAlias()
  const deleteAPIKeyAlias = useDeleteAPIKeyAlias()
  const savePricing = useSavePricing()
  const toast = useToast()
  const [query, setQuery] = useState("")
  const [keyAliasScope, setKeyAliasScope] = useState<KeyAliasScope>("api-key")
  const [editingId, setEditingId] = useState<string | null>(null)
  const [draftAlias, setDraftAlias] = useState("")
  const [drafts, setDrafts] = useState<CostRateDrafts>({})
  const [savingModel, setSavingModel] = useState<string | null>(null)

  const apiKeyRows: ReferenceKeyRow[] = useMemo(() => normalizeAPIKeyRows(apiKeys ?? []), [apiKeys])

  const accountRows: ReferenceKeyRow[] = useMemo(() => normalizeAccountKeyRows(keys ?? []), [keys])

  const visibleRows = keyAliasScope === "api-key" ? apiKeyRows : accountRows
  const isAliasLoading = keyAliasScope === "api-key" ? isAPIKeysLoading : isKeysLoading

  const filteredKeys = useMemo(() => filterKeyAliasRows(visibleRows, query), [visibleRows, query])

  const pricing = useMemo(() => pricingData?.pricing ?? [], [pricingData?.pricing])
  const pricingMap = useMemo(() => buildPricingMap(pricing), [pricing])
  const models = useMemo(() => buildCostRateModels(pricingData?.usedModels ?? [], pricing), [pricingData?.usedModels, pricing])
  const missingRates = countMissingCostRates(models, pricingMap)
  const aliasedAPIKeys = countAliasedRows(apiKeyRows)
  const aliasedAccounts = countAliasedRows(accountRows)

  function startEdit(key: ReferenceKeyRow) {
    setEditingId(key.id)
    setDraftAlias(key.alias)
  }

  async function saveEdit(key: ReferenceKeyRow) {
    if (!canSaveKeyAlias(draftAlias)) {
      toast.error("Use clear to remove an alias")
      return
    }
    try {
      if (keyAliasScope === "api-key") {
        await updateAPIKeyAlias.mutateAsync({ id: key.id, alias: draftAlias })
      } else {
        await updateAlias.mutateAsync({ id: Number(key.id), alias: draftAlias })
      }
      setEditingId(null)
      toast.success("Alias saved")
    } catch {
      toast.error("Failed to save alias")
    }
  }

  async function clearEdit(key: ReferenceKeyRow) {
    try {
      if (keyAliasScope === "api-key") {
        await deleteAPIKeyAlias.mutateAsync(key.id)
      } else {
        await deleteAlias.mutateAsync(Number(key.id))
      }
      setEditingId(null)
      toast.success("Alias cleared")
    } catch {
      toast.error("Failed to clear alias")
    }
  }

  function getDraft(model: string) {
    return getCostRateDraft(model, pricingMap, drafts)
  }

  function updateDraft(model: string, field: keyof CostRateDrafts[string], value: string) {
    setDrafts((prev) => ({
      ...prev,
      [model]: { ...getDraft(model), [field]: value },
    }))
  }

  async function saveRate(model: string) {
    const validation = validateCostRateDraft(getDraft(model))
    if (!validation.valid) {
      toast.error(validation.reason === "missing" ? "Enter all rates before saving" : "Rates must be non-negative numbers")
      return
    }

    setSavingModel(model)
    try {
      await savePricing.mutateAsync(buildCostRateSavePayload(model, validation.prices))
      toast.success(`${model} cost rate saved`)
    } catch {
      toast.error("Failed to save cost rate")
    } finally {
      setSavingModel(null)
    }
  }

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
        <SummaryCard label="API Keys" value={apiKeys?.length ?? 0} caption={`${aliasedAPIKeys} aliased`} loading={isAPIKeysLoading} />
        <SummaryCard label="Accounts" value={keys?.length ?? 0} caption={`${aliasedAccounts} aliased`} loading={isKeysLoading} />
        <SummaryCard label="Missing Cost Rates" value={missingRates} caption="Models without configured rates" loading={isPricingLoading} tone={missingRates > 0 ? "amber" : "green"} />
      </div>

      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-4">
          <div>
            <CardTitle>Key Aliases</CardTitle>
            <CardDescription>{keyAliasScope === "api-key" ? "Human-readable labels for raw API keys" : "Human-readable labels for account keys"}</CardDescription>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <div className="flex items-center rounded-lg border border-border bg-card p-1">
              {[
                { value: "api-key", label: "API Keys" },
                { value: "account", label: "Accounts" },
              ].map((item) => (
                <button
                  key={item.value}
                  onClick={() => {
                    setKeyAliasScope(item.value as KeyAliasScope)
                    setEditingId(null)
                  }}
                  aria-label={`Key alias scope: ${item.label}`}
                  aria-pressed={keyAliasScope === item.value}
                  className={cn(
                    "rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
                    keyAliasScope === item.value
                      ? "bg-terracotta-500 text-white"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-2 rounded-lg border border-border bg-background px-3 py-2">
              <Search className="h-4 w-4 text-muted-foreground" />
              <input
                name="key-alias-search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search alias or key..."
                className="min-w-[200px] bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {isAliasLoading ? (
              <>
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </>
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
                    className="grid items-center gap-3 rounded-lg border border-border p-3 sm:grid-cols-[1fr_120px_130px_100px]"
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

      <Card>
        <CardHeader>
          <CardTitle>Cost Rates</CardTitle>
          <CardDescription>Model unit rates used by Cost calculations</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {isPricingLoading ? (
              <>
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </>
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
                    className="grid items-end gap-3 rounded-lg border border-border p-3 sm:grid-cols-[1fr_120px_120px_120px_auto]"
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

function SummaryCard({
  label,
  value,
  caption,
  loading,
  tone = "terracotta",
}: {
  label: string
  value: number | string
  caption: string
  loading: boolean
  tone?: "terracotta" | "green" | "amber"
}) {
  const toneClass = {
    terracotta: "text-terracotta-700",
    green: "text-emerald-700",
    amber: "text-amber-700",
  }[tone]

  return (
    <Card>
      <CardContent className="p-5">
        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-8 w-20" />
          </div>
        ) : (
          <>
            <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{label}</p>
            <p className={`mt-2 font-serif text-2xl font-semibold tracking-tight ${toneClass}`}>{value}</p>
            <p className="mt-1 text-xs text-muted-foreground">{caption}</p>
          </>
        )}
      </CardContent>
    </Card>
  )
}

function RateInput({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <label className="space-y-1">
      <span className="text-[10px] font-medium text-muted-foreground">{label}</span>
      <input
        type="number"
        min="0"
        step="0.000001"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="-"
        className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
      />
    </label>
  )
}
