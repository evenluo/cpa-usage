import { useMemo, useState } from "react"
import { useToast } from "@/components/providers/toast-provider"
import { useAPIKeys, useDeleteAPIKeyAlias, useDeleteAlias, useKeys, useUpdateAPIKeyAlias, useUpdateAlias } from "@/hooks/useKeys"
import { usePricing, useSavePricing } from "@/hooks/usePricing"
import type { PricingEntry } from "@/types/api"
import {
  beginKeyAliasDraft,
  buildCostRateSaveCommand,
  buildCostRateModels,
  buildKeyAliasClearCommand,
  buildKeyAliasSaveCommand,
  buildPricingMap,
  countAliasedRows,
  countMissingCostRates,
  filterKeyAliasRows,
  getCostRateDraft,
  keyAliasScopeDescription,
  nextCostRateDrafts,
  normalizeAccountKeyRows,
  normalizeAPIKeyRows,
  selectKeyAliasRows,
  type CostRateDraft,
  type CostRateDrafts,
  type KeyAliasScope,
  type ReferenceKeyRow,
} from "./model"

export interface UseReferenceDataWorkbenchResult {
  query: string
  setQuery: (query: string) => void
  keyAliasScope: KeyAliasScope
  selectKeyAliasScope: (scope: KeyAliasScope) => void
  scopeDescription: string
  apiKeyCount?: number
  accountCount?: number
  aliasedAPIKeys?: number
  aliasedAccounts?: number
  missingRates?: number
  apiKeysRead: ReferenceReadState
  accountsRead: ReferenceReadState
  pricingRead: ReferenceReadState
  aliasRead: ReferenceReadState
  filteredKeys: ReferenceKeyRow[]
  editingId: string | null
  draftAlias: string
  setDraftAlias: (alias: string) => void
  startEdit: (key: ReferenceKeyRow) => void
  cancelEdit: () => void
  saveEdit: (key: ReferenceKeyRow) => Promise<void>
  clearEdit: (key: ReferenceKeyRow) => Promise<void>
  models: string[]
  pricingMap: Map<string, PricingEntry>
  getDraft: (model: string) => CostRateDraft
  updateDraft: (model: string, field: keyof CostRateDraft, value: string) => void
  saveRate: (model: string) => Promise<void>
  savingModel: string | null
}

export type ReferenceReadStatus = "loading" | "error" | "empty" | "ready"

export interface ReferenceReadState {
  status: ReferenceReadStatus
  refreshError?: unknown
  retry: () => void
}

function buildReferenceReadState<T>(input: {
  data: T | undefined
  isLoading: boolean
  error: unknown
  isEmpty: (data: T) => boolean
  retry: () => void
}): ReferenceReadState {
  if (input.data === undefined) {
    if (input.isLoading) return { status: "loading", retry: input.retry }
    if (input.error) return { status: "error", retry: input.retry }
    return { status: "empty", retry: input.retry }
  }
  const status = input.isEmpty(input.data) ? "empty" : "ready"
  return input.error
    ? { status, refreshError: input.error, retry: input.retry }
    : { status, retry: input.retry }
}

/**
 * Owns the Reference Data page state: Key Alias / Cost Rate edit drafts,
 * derived workbench rows, and save/clear command wiring.
 * Route files consume the returned state and setters for composition only.
 */
export function useReferenceDataWorkbench(): UseReferenceDataWorkbenchResult {
  const keysQuery = useKeys()
  const apiKeysQuery = useAPIKeys()
  const pricingQuery = usePricing()
  const { data: keys } = keysQuery
  const { data: apiKeys } = apiKeysQuery
  const { data: pricingData } = pricingQuery
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
  const visibleRows = selectKeyAliasRows(keyAliasScope, apiKeyRows, accountRows)
  const filteredKeys = useMemo(() => filterKeyAliasRows(visibleRows, query), [visibleRows, query])

  const pricing = useMemo(() => pricingData?.pricing ?? [], [pricingData?.pricing])
  const pricingMap = useMemo(() => buildPricingMap(pricing), [pricing])
  const models = useMemo(() => buildCostRateModels(pricingData?.usedModels ?? [], pricing), [pricingData?.usedModels, pricing])
  const missingRates = countMissingCostRates(models, pricingMap)
  const aliasedAPIKeys = countAliasedRows(apiKeyRows)
  const aliasedAccounts = countAliasedRows(accountRows)
  const apiKeysRead = buildReferenceReadState({
    data: apiKeys,
    isLoading: apiKeysQuery.isLoading,
    error: apiKeysQuery.error,
    isEmpty: (rows) => rows.length === 0,
    retry: () => void apiKeysQuery.refetch(),
  })
  const accountsRead = buildReferenceReadState({
    data: keys,
    isLoading: keysQuery.isLoading,
    error: keysQuery.error,
    isEmpty: (rows) => rows.length === 0,
    retry: () => void keysQuery.refetch(),
  })
  const pricingRead = buildReferenceReadState({
    data: pricingData,
    isLoading: pricingQuery.isLoading,
    error: pricingQuery.error,
    isEmpty: () => models.length === 0,
    retry: () => void pricingQuery.refetch(),
  })
  const aliasRead = keyAliasScope === "api-key" ? apiKeysRead : accountsRead

  function selectKeyAliasScope(scope: KeyAliasScope) {
    setKeyAliasScope(scope)
    setEditingId(null)
  }

  function startEdit(key: ReferenceKeyRow) {
    const draft = beginKeyAliasDraft(key)
    setEditingId(draft.editingId)
    setDraftAlias(draft.draftAlias)
  }

  function cancelEdit() {
    setEditingId(null)
  }

  async function saveEdit(key: ReferenceKeyRow) {
    const command = buildKeyAliasSaveCommand(keyAliasScope, key, draftAlias)
    if (!command.valid) {
      toast.error("Use clear to remove an alias")
      return
    }
    try {
      if (command.scope === "api-key") {
        await updateAPIKeyAlias.mutateAsync({ id: command.id, alias: command.alias })
      } else {
        await updateAlias.mutateAsync({ id: command.id, alias: command.alias })
      }
      setEditingId(null)
      toast.success("Alias saved")
    } catch {
      toast.error("Failed to save alias")
    }
  }

  async function clearEdit(key: ReferenceKeyRow) {
    const command = buildKeyAliasClearCommand(keyAliasScope, key)
    if (!command.valid) {
      toast.error("Failed to clear alias")
      return
    }
    try {
      if (command.scope === "api-key") {
        await deleteAPIKeyAlias.mutateAsync(command.id)
      } else {
        await deleteAlias.mutateAsync(command.id)
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

  function updateDraft(model: string, field: keyof CostRateDraft, value: string) {
    setDrafts((prev) => nextCostRateDrafts(model, pricingMap, prev, field, value))
  }

  async function saveRate(model: string) {
    const command = buildCostRateSaveCommand(model, getDraft(model))
    if (!command.valid) {
      toast.error(command.reason === "missing" ? "Enter all rates before saving" : "Rates must be non-negative numbers")
      return
    }

    setSavingModel(model)
    try {
      await savePricing.mutateAsync(command.payload)
      toast.success(`${model} cost rate saved`)
    } catch {
      toast.error("Failed to save cost rate")
    } finally {
      setSavingModel(null)
    }
  }

  return {
    query,
    setQuery,
    keyAliasScope,
    selectKeyAliasScope,
    scopeDescription: keyAliasScopeDescription(keyAliasScope),
    apiKeyCount: apiKeys?.length,
    accountCount: keys?.length,
    aliasedAPIKeys: apiKeys === undefined ? undefined : aliasedAPIKeys,
    aliasedAccounts: keys === undefined ? undefined : aliasedAccounts,
    missingRates: pricingData === undefined ? undefined : missingRates,
    apiKeysRead,
    accountsRead,
    pricingRead,
    aliasRead,
    filteredKeys,
    editingId,
    draftAlias,
    setDraftAlias,
    startEdit,
    cancelEdit,
    saveEdit,
    clearEdit,
    models,
    pricingMap,
    getDraft,
    updateDraft,
    saveRate,
    savingModel,
  }
}
