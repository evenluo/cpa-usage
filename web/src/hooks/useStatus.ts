import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { apiFetch, metricsFetch } from "@/lib/api"
import type { MetricsPayload, StatusPayload } from "@/types/api"

const STATUS_QUERY_KEY = ["status"] as const
const METRICS_QUERY_KEY = ["metrics"] as const

/**
 * Prefixes refreshed after a successful manual sync.
 * Maps to CONTEXT.md: usage, evidence, identity, and reference-data read models.
 *
 * Excluded on purpose:
 * - ["status"] is refreshed onSettled so operators see sync state after success or failure
 * - ["quota"] is Live Capacity quota observations, not a usage-event read model
 * - ["auth"] is session state, not a sync read model
 */
export const MANUAL_SYNC_READ_MODEL_QUERY_KEYS = [
  ["analytics"],
  ["usage"],
  ["events"],
  ["keys"],
  ["pricing"],
] as const

export function useStatus() {
  return useQuery({
    queryKey: STATUS_QUERY_KEY,
    queryFn: () => apiFetch<StatusPayload>("/status"),
    staleTime: 30_000,
  })
}

export function useMetrics() {
	return useQuery({
		queryKey: METRICS_QUERY_KEY,
		queryFn: () => metricsFetch<MetricsPayload>(),
		staleTime: 30_000,
	})
}

export function useManualSync() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch("/sync", { method: "POST" }),
    onSuccess: () => {
      for (const queryKey of MANUAL_SYNC_READ_MODEL_QUERY_KEYS) {
        qc.invalidateQueries({ queryKey: [...queryKey] })
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: STATUS_QUERY_KEY })
    },
  })
}
