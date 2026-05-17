import { createLazyFileRoute } from "@tanstack/react-router"
import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { usePricing, useSavePricing } from "@/hooks/usePricing"
import { useToast } from "@/components/providers/toast-provider"

export const Route = createLazyFileRoute("/pricing")({
  component: PricingPage,
})

interface Drafts {
  [model: string]: {
    prompt: string
    completion: string
    cache: string
  }
}

function PricingPage() {
  const { data, isLoading } = usePricing()
  const savePricing = useSavePricing()
  const toast = useToast()
  const [drafts, setDrafts] = useState<Drafts>({})
  const [savingModel, setSavingModel] = useState<string | null>(null)

  const pricingMap = new Map(data?.pricing.map((p) => [p.model, p]))
  const allModels = Array.from(
    new Set([...(data?.usedModels ?? []), ...(data?.pricing.map((p) => p.model) ?? [])])
  ).sort()

  function getDraft(model: string) {
    const existing = pricingMap.get(model)
    return (
      drafts[model] ?? {
        prompt: String(existing?.prompt_price_per_1m ?? 0),
        completion: String(existing?.completion_price_per_1m ?? 0),
        cache: String(existing?.cache_price_per_1m ?? 0),
      }
    )
  }

  function updateDraft(model: string, field: keyof Drafts[string], value: string) {
    setDrafts((prev) => ({
      ...prev,
      [model]: { ...getDraft(model), [field]: value },
    }))
  }

  async function save(model: string) {
    const draft = getDraft(model)
    const prices = [draft.prompt, draft.completion, draft.cache].map(Number)
    if (prices.some((p) => !Number.isFinite(p) || p < 0)) {
      toast.error("Prices must be non-negative numbers")
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
      toast.success(`${model} pricing saved`)
    } catch {
      toast.error("Failed to save pricing")
    } finally {
      setSavingModel(null)
    }
  }

  return (
    <div className="animate-slide-up space-y-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Model Pricing</p>
          <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight">Pricing</h1>
        </div>
        <Badge variant={allModels.some((m) => !pricingMap.has(m)) ? "amber" : "green"}>
          {allModels.filter((m) => !pricingMap.has(m)).length} missing
        </Badge>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Configured Cost Rates</CardTitle>
          <CardDescription>Model unit prices used by analytics cost calculations</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {isLoading ? (
              <>
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </>
            ) : allModels.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
                No models available for pricing
              </div>
            ) : (
              allModels.map((model) => {
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
                        {configured ? "Configured" : "Missing price"}
                      </Badge>
                    </div>
                    <label className="space-y-1">
                      <span className="text-[10px] font-medium text-muted-foreground">Prompt</span>
                      <input
                        type="number"
                        min="0"
                        step="0.000001"
                        value={draft.prompt}
                        onChange={(e) => updateDraft(model, "prompt", e.target.value)}
                        className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
                      />
                    </label>
                    <label className="space-y-1">
                      <span className="text-[10px] font-medium text-muted-foreground">Completion</span>
                      <input
                        type="number"
                        min="0"
                        step="0.000001"
                        value={draft.completion}
                        onChange={(e) => updateDraft(model, "completion", e.target.value)}
                        className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
                      />
                    </label>
                    <label className="space-y-1">
                      <span className="text-[10px] font-medium text-muted-foreground">Cache</span>
                      <input
                        type="number"
                        min="0"
                        step="0.000001"
                        value={draft.cache}
                        onChange={(e) => updateDraft(model, "cache", e.target.value)}
                        className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
                      />
                    </label>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-9"
                      disabled={savingModel === model}
                      onClick={() => void save(model)}
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
