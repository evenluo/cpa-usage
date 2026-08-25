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
  apiKeyCount: number
  accountCount: number
  aliasedAPIKeys: number
  aliasedAccounts: number
  missingRates: number
  isAPIKeysLoading: boolean
  isKeysLoading: boolean
  isPricingLoading: boolean
  isAliasLoading: boolean
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

/**
 * Owns the Reference Data page state: Key Alias / Cost Rate edit drafts,
 * derived workbench rows, and save/clear command wiring.
 * Route files consume the returned state and setters for composition only.
 */
export function useReferenceDataWorkbench(): UseReferenceDataWorkbenchResult {
  const { data: keys, isLoading: isKeysLoading } = useKeys()
  const { data: apiKeys, isLoading: isAPIKeysLoading } = useAPIKeys()
  const { data: pricingData, isLoading: isPricingLoading } = usePricing()
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
  const isAliasLoading = keyAliasScope === "api-key" ? isAPIKeysLoading : isKeysLoading
  const filteredKeys = useMemo(() => filterKeyAliasRows(visibleRows, query), [visibleRows, query])

  const pricing = useMemo(() => pricingData?.pricing ?? [], [pricingData?.pricing])
  const pricingMap = useMemo(() => buildPricingMap(pricing), [pricing])
  const models = useMemo(() => buildCostRateModels(pricingData?.usedModels ?? [], pricing), [pricingData?.usedModels, pricing])
  const missingRates = countMissingCostRates(models, pricingMap)
  const aliasedAPIKeys = countAliasedRows(apiKeyRows)
  const aliasedAccounts = countAliasedRows(accountRows)

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
    apiKeyCount: apiKeys?.length ?? 0,
    accountCount: keys?.length ?? 0,
    aliasedAPIKeys,
    aliasedAccounts,
    missingRates,
    isAPIKeysLoading,
    isKeysLoading,
    isPricingLoading,
    isAliasLoading,
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
