import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ApiError, apiFetch } from "@/lib/api"
import { collectPaginatedItems } from "@/lib/pagination"
import type { APIKeyAliasTarget, APIKeyAliasTargetPage, KeyIdentityPage, KeyIdentity } from "@/types/api"

const PAGE_SIZE = 100

function accountToggleError(message: string, cause: unknown): Error {
  const error = new Error(message)
  Object.defineProperty(error, "cause", { configurable: true, value: cause })
  return error
}

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
      try {
        const res = await apiFetch<{ disabled: boolean }>(`/usage/identities/${id}/disabled`, {
          method: "PUT",
          body: JSON.stringify({ disabled }),
        })
        return res
      } catch (error) {
        if (!(error instanceof ApiError)) throw accountToggleError("Failed to update account", error)
        let body: unknown
        try {
          body = JSON.parse(error.body)
        } catch {
          throw accountToggleError("Failed to update account", error)
        }
        if (typeof body === "object" && body !== null && "error" in body && typeof body.error === "string") {
          throw accountToggleError(body.error, error)
        }
        throw accountToggleError("Failed to update account", error)
      }
    },
    onSettled: () => {
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
