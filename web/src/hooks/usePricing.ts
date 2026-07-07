import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { PricingPayload, PricingEntry, UsedModelsPayload } from "@/types/api"

export function usePricing() {
  return useQuery({
    queryKey: ["pricing"],
    queryFn: async () => {
      const [pricing, used] = await Promise.all([
        apiFetch<PricingPayload>("/pricing"),
        apiFetch<UsedModelsPayload>("/models/used"),
      ])
      return { pricing: pricing.pricing ?? [], usedModels: used.models ?? [] }
    },
    staleTime: 60_000,
  })
}

export function useSavePricing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (entry: PricingEntry) => {
      return apiFetch<PricingEntry>("/pricing", {
        method: "PUT",
        body: JSON.stringify({
          model: entry.model,
          prompt_price_per_1m: entry.prompt_price_per_1m,
          completion_price_per_1m: entry.completion_price_per_1m,
          cache_price_per_1m: entry.cache_price_per_1m,
        }),
      })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["pricing"] })
      qc.invalidateQueries({ queryKey: ["analytics"] })
    },
  })
}
