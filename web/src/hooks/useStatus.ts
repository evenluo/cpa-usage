import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { StatusPayload } from "@/types/api"

export function useStatus() {
  return useQuery({
    queryKey: ["status"],
    queryFn: () => apiFetch<StatusPayload>("/status"),
    staleTime: 30_000,
  })
}
