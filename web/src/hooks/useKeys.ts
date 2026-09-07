import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import { collectPaginatedItems } from "@/lib/pagination"
import type { APIKeyAliasTarget, APIKeyAliasTargetPage, KeyIdentityPage, KeyIdentity } from "@/types/api"

const PAGE_SIZE = 100

async function fetchPage(page: number): Promise<KeyIdentityPage> {
  return apiFetch(`/usage/identities/page?page=${page}&page_size=${PAGE_SIZE}`)
}

async function fetchAPIKeyPage(page: number): Promise<APIKeyAliasTargetPage> {
  return apiFetch(`/usage/api-keys/page?page=${page}&page_size=${PAGE_SIZE}`)
}

export async function fetchAllKeys(): Promise<KeyIdentity[]> {
  return collectPaginatedItems({
    fetchPage,
    getItems: (page) => page.identities,
    resource: "Accounts",
    expectedPageSize: PAGE_SIZE,
  })
}

export async function fetchAllAPIKeys(): Promise<APIKeyAliasTarget[]> {
  return collectPaginatedItems({
    fetchPage: fetchAPIKeyPage,
    getItems: (page) => page.api_keys,
    resource: "API keys",
    expectedPageSize: PAGE_SIZE,
  })
}

export function useKeys() {
  return useQuery({
    queryKey: ["keys", "identities"],
    queryFn: fetchAllKeys,
    staleTime: 60_000,
  })
}

export function useAPIKeys() {
  return useQuery({
    queryKey: ["keys", "api-keys"],
    queryFn: fetchAllAPIKeys,
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
      qc.invalidateQueries({ queryKey: ["keys", "api-keys"] })
      qc.invalidateQueries({ queryKey: ["analytics"] })
    },
  })
}

export function useSetIdentityDisabled() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, disabled }: { id: number; disabled: boolean }) => {
      const res = await apiFetch<{ disabled: boolean }>(`/usage/identities/${id}/disabled`, {
        method: "PUT",
        body: JSON.stringify({ disabled }),
      })
      return res
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["quota", "auth-file-identities"] })
      qc.invalidateQueries({ queryKey: ["keys", "identities"] })
    },
  })
}

export function useUpdateAPIKeyAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, alias }: { id: string; alias: string }) => {
      const res = await apiFetch<{ alias: string }>(`/usage/api-keys/${encodeURIComponent(id)}/alias`, {
        method: "PUT",
        body: JSON.stringify({ alias }),
      })
      return res
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["keys", "api-keys"] })
      qc.invalidateQueries({ queryKey: ["analytics"] })
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
      qc.invalidateQueries({ queryKey: ["keys", "api-keys"] })
      qc.invalidateQueries({ queryKey: ["analytics"] })
    },
  })
}

export function useDeleteAPIKeyAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      await apiFetch(`/usage/api-keys/${encodeURIComponent(id)}/alias`, { method: "DELETE" })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["keys", "api-keys"] })
      qc.invalidateQueries({ queryKey: ["analytics"] })
    },
  })
}
