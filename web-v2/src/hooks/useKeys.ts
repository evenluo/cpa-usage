import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { KeyIdentityPage, KeyIdentity } from "@/types/api"

const PAGE_SIZE = 100

async function fetchPage(page: number): Promise<KeyIdentityPage> {
  return apiFetch(`/usage/identities/page?page=${page}&page_size=${PAGE_SIZE}`)
}

async function fetchAllKeys(): Promise<KeyIdentity[]> {
  const first = await fetchPage(1)
  const totalPages = Math.max(1, Math.trunc(first.total_pages ?? 1))
  if (totalPages <= 1) return first.identities ?? []
  const rest = await Promise.all(
    Array.from({ length: totalPages - 1 }, (_, i) => fetchPage(i + 2))
  )
  return [first, ...rest].flatMap((p) => p.identities ?? [])
}

export function useKeys() {
  return useQuery({
    queryKey: ["keys", "identities"],
    queryFn: fetchAllKeys,
    staleTime: 60_000,
  })
}

export function useUpdateAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, alias }: { id: number; alias: string }) => {
      const res = await apiFetch<{ alias: string }>(`/usage/identities/${id}/alias`, {
        method: "PUT",
        body: JSON.stringify({ alias }),
      })
      return res
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["keys", "identities"] })
      qc.invalidateQueries({ queryKey: ["analytics", "summary"] })
    },
  })
}

export function useDeleteAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: number) => {
      await apiFetch(`/usage/identities/${id}/alias`, { method: "DELETE" })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["keys", "identities"] })
      qc.invalidateQueries({ queryKey: ["analytics", "summary"] })
    },
  })
}
