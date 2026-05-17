import { createLazyFileRoute } from "@tanstack/react-router"
import { useMemo, useState } from "react"
import { Check, Pencil, Search, Trash2, X } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useToast } from "@/components/providers/toast-provider"
import { useDeleteAlias, useKeys, useUpdateAlias } from "@/hooks/useKeys"
import { usePricing, useSavePricing } from "@/hooks/usePricing"
import { formatCompact, formatCost, formatDate } from "@/lib/format"

export const Route = createLazyFileRoute("/reference")({
  component: ReferencePage,
})

interface Drafts {
  [model: string]: {
    prompt: string
    completion: string
    cache: string
  }
}

function ReferencePage() {
  const { data: keys, isLoading: isKeysLoading } = useKeys()
  const { data: pricingData, isLoading: isPricingLoading } = usePricing()
  const updateAlias = useUpdateAlias()
  const deleteAlias = useDeleteAlias()
  const savePricing = useSavePricing()
  const toast = useToast()
  const [query, setQuery] = useState("")
  const [editingId, setEditingId] = useState<number | null>(null)
  const [draftAlias, setDraftAlias] = useState("")
  const [drafts, setDrafts] = useState<Drafts>({})
  const [savingModel, setSavingModel] = useState<string | null>(null)

  const filteredKeys = useMemo(() => {
    if (!keys) return []
    const q = query.trim().toLowerCase()
    if (!q) return keys
    return keys.filter((key) =>
      [key.alias, key.displayName, key.name, key.identity, key.provider, key.type, key.auth_type_name]
        .some((value) => value?.toLowerCase().includes(q))
    )
  }, [keys, query])

  const pricingMap = new Map(pricingData?.pricing.map((entry) => [entry.model, entry]))
  const models = Array.from(
    new Set([...(pricingData?.usedModels ?? []), ...(pricingData?.pricing.map((entry) => entry.model) ?? [])])
  ).sort((a, b) => {
    const aMissing = pricingMap.has(a) ? 1 : 0
    const bMissing = pricingMap.has(b) ? 1 : 0
    return aMissing - bMissing || a.localeCompare(b)
  })
  const missingRates = models.filter((model) => !pricingMap.has(model)).length
  const aliasedKeys = keys?.filter((key) => key.alias).length ?? 0

  function startEdit(key: typeof filteredKeys[0]) {
    setEditingId(key.id)
    setDraftAlias(key.alias)
  }

  async function saveEdit(key: typeof filteredKeys[0]) {
    if (draftAlias.trim() === "") {
      toast.error("Use clear to remove an alias")
      return
    }
    try {
      await updateAlias.mutateAsync({ id: key.id, alias: draftAlias })
      setEditingId(null)
      toast.success("Alias saved")
    } catch {
      toast.error("Failed to save alias")
    }
  }

  async function clearEdit(key: typeof filteredKeys[0]) {
    try {
      await deleteAlias.mutateAsync(key.id)
      setEditingId(null)
      toast.success("Alias cleared")
    } catch {
      toast.error("Failed to clear alias")
    }
  }

  function getDraft(model: string) {
    const existing = pricingMap.get(model)
    return (
      drafts[model] ?? {
        prompt: existing ? String(existing.prompt_price_per_1m) : "",
        completion: existing ? String(existing.completion_price_per_1m) : "",
        cache: existing ? String(existing.cache_price_per_1m) : "",
      }
    )
  }

  function updateDraft(model: string, field: keyof Drafts[string], value: string) {
    setDrafts((prev) => ({
      ...prev,
      [model]: { ...getDraft(model), [field]: value },
    }))
  }

  async function saveRate(model: string) {
    const draft = getDraft(model)
    if ([draft.prompt, draft.completion, draft.cache].some((value) => value.trim() === "")) {
      toast.error("Enter all rates before saving")
      return
    }
    const prices = [draft.prompt, draft.completion, draft.cache].map(Number)
    if (prices.some((price) => !Number.isFinite(price) || price < 0)) {
      toast.error("Rates must be non-negative numbers")
      return
    }

    setSavingModel(model)
    try {
      await savePricing.mutateAsync({
        model,
        prompt_price_per_1m: prices[0],
        completion_price_per_1m: prices[1],
        cache_price_per_1m: prices[2],
      })
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
        <SummaryCard label="CPA Keys" value={keys?.length ?? 0} caption={`${aliasedKeys} aliased`} loading={isKeysLoading} />
        <SummaryCard label="Missing Cost Rates" value={missingRates} caption="Models without configured rates" loading={isPricingLoading} tone={missingRates > 0 ? "amber" : "green"} />
        <SummaryCard label="Reference Health" value={missingRates > 0 ? "Partial" : "Complete"} caption="Used by Cost and leaderboards" loading={isPricingLoading} tone={missingRates > 0 ? "amber" : "green"} />
      </div>

      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-4">
          <div>
            <CardTitle>Key Aliases</CardTitle>
            <CardDescription>Human-readable labels for CPA Keys</CardDescription>
          </div>
          <div className="flex items-center gap-2 rounded-lg border border-border bg-background px-3 py-2">
            <Search className="h-4 w-4 text-muted-foreground" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search alias or key..."
              className="min-w-[200px] bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {isKeysLoading ? (
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
