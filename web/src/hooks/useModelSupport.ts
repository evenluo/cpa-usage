import { useMutation } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"
import type { ModelSupportResponse } from "@/types/api"

export const MODEL_SUPPORT_MAX_ACCOUNTS = 12

export async function loadModelSupport(identityIds: number[]): Promise<ModelSupportResponse> {
  if (identityIds.length === 0) {
    throw new Error("Select at least one account")
  }
  if (identityIds.length > MODEL_SUPPORT_MAX_ACCOUNTS) {
    throw new Error(`Narrow selection to ${MODEL_SUPPORT_MAX_ACCOUNTS} accounts or fewer`)
  }
  if (identityIds.some((id) => !Number.isInteger(id) || id <= 0) || new Set(identityIds).size !== identityIds.length) {
    throw new Error("Account selection is invalid")
  }
  return apiFetch("/usage/identities/model-support", {
    method: "POST",
    body: JSON.stringify({ identity_ids: identityIds }),
  })
}

// Mutation-only by design: model support is fetched solely from an explicit
// Load action and is never a page-load query or a permanent client cache.
export function useModelSupport() {
  return useMutation({ mutationFn: loadModelSupport })
}
