import type { APIKeyAliasTarget, KeyIdentity, PricingEntry } from "@/types/api"

export interface CostRateDraft {
  prompt: string
  completion: string
  cache: string
}

export type CostRateDrafts = Record<string, CostRateDraft>
export type KeyAliasScope = "api-key" | "account"
export type KeyAliasMutationCommand =
  | { valid: true; scope: "api-key"; id: string; alias: string }
  | { valid: true; scope: "account"; id: number; alias: string }
  | { valid: false; reason: "empty" | "invalid-account-id" }
export type CostRateSaveCommand =
  | { valid: true; payload: PricingEntry }
  | { valid: false; reason: "missing" | "invalid" }

export const KEY_ALIAS_SCOPES: Array<{ value: KeyAliasScope; label: string; description: string }> = [
  { value: "api-key", label: "API Keys", description: "Human-readable labels for raw API keys" },
  { value: "account", label: "Accounts", description: "Human-readable labels for account keys" },
]

export interface ReferenceKeyRow {
  id: string
  alias: string
  displayName: string
  name: string
  identity: string
  provider: string
  type: string
  auth_type_name: string
  total_tokens: number
  total_cost: number
  cost_available: boolean
  last_used_at: string | null
}

export type CostRateValidation =
  | { valid: true; prices: [number, number, number] }
  | { valid: false; reason: "missing" | "invalid" }

export function normalizeAPIKeyRows(apiKeys: APIKeyAliasTarget[] = []): ReferenceKeyRow[] {
  return apiKeys.map((key) => ({
    id: key.id,
    alias: key.alias,
    displayName: key.displayName,
    name: "",
    identity: key.identity,
    provider: key.provider,
    type: "api-key",
    auth_type_name: key.auth_type_name,
    total_tokens: key.total_tokens,
    total_cost: key.total_cost,
    cost_available: key.cost_available,
    last_used_at: key.last_used_at ?? null,
  }))
}

export function normalizeAccountKeyRows(keys: KeyIdentity[] = []): ReferenceKeyRow[] {
  return keys.map((key) => ({
    id: String(key.id),
    alias: key.alias,
    displayName: key.displayName,
    name: key.name,
    identity: key.identity,
    provider: key.provider,
    type: key.type,
    auth_type_name: key.auth_type_name,
    total_tokens: key.total_tokens,
    total_cost: key.total_cost,
    cost_available: key.cost_available,
    last_used_at: key.last_used_at,
  }))
}

export function filterKeyAliasRows(rows: ReferenceKeyRow[], query: string): ReferenceKeyRow[] {
  const q = query.trim().toLowerCase()
  if (!q) return rows
  return rows.filter((key) =>
    [key.alias, key.displayName, key.name, key.identity, key.provider, key.type, key.auth_type_name]
      .some((value) => value?.toLowerCase().includes(q)),
  )
}

export function selectKeyAliasRows(
  scope: KeyAliasScope,
  apiKeyRows: ReferenceKeyRow[],
  accountRows: ReferenceKeyRow[],
): ReferenceKeyRow[] {
  return scope === "api-key" ? apiKeyRows : accountRows
}

export function keyAliasScopeDescription(scope: KeyAliasScope): string {
  return KEY_ALIAS_SCOPES.find((item) => item.value === scope)?.description ?? ""
}

export function beginKeyAliasDraft(row: ReferenceKeyRow): { editingId: string; draftAlias: string } {
  return { editingId: row.id, draftAlias: row.alias }
}

export function canSaveKeyAlias(alias: string): boolean {
  return alias.trim() !== ""
}

export function buildKeyAliasSaveCommand(
  scope: KeyAliasScope,
  row: ReferenceKeyRow,
  alias: string,
): KeyAliasMutationCommand {
  if (!canSaveKeyAlias(alias)) {
    return { valid: false, reason: "empty" }
  }
  if (scope === "api-key") {
    return { valid: true, scope, id: row.id, alias }
  }
  const id = Number(row.id)
  if (!Number.isFinite(id)) {
    return { valid: false, reason: "invalid-account-id" }
  }
  return { valid: true, scope, id, alias }
}

export function buildKeyAliasClearCommand(scope: KeyAliasScope, row: ReferenceKeyRow): KeyAliasMutationCommand {
  if (scope === "api-key") {
    return { valid: true, scope, id: row.id, alias: "" }
  }
  const id = Number(row.id)
  if (!Number.isFinite(id)) {
    return { valid: false, reason: "invalid-account-id" }
  }
  return { valid: true, scope, id, alias: "" }
}

export function countAliasedRows(rows: Array<{ alias: string }>): number {
  return rows.filter((row) => row.alias).length
}

export function buildPricingMap(pricing: PricingEntry[] = []): Map<string, PricingEntry> {
  return new Map(pricing.map((entry) => [entry.model, entry]))
}

export function buildCostRateModels(usedModels: string[] = [], pricing: PricingEntry[] = []): string[] {
  const pricingMap = buildPricingMap(pricing)
  return Array.from(new Set([...usedModels, ...pricing.map((entry) => entry.model)])).sort((a, b) => {
    const aMissing = pricingMap.has(a) ? 1 : 0
    const bMissing = pricingMap.has(b) ? 1 : 0
    return aMissing - bMissing || a.localeCompare(b)
  })
}

export function countMissingCostRates(models: string[], pricingMap: Map<string, PricingEntry>): number {
  return models.filter((model) => !pricingMap.has(model)).length
}

export function getCostRateDraft(
  model: string,
  pricingMap: Map<string, PricingEntry>,
  drafts: CostRateDrafts,
): CostRateDraft {
  const existing = pricingMap.get(model)
  return drafts[model] ?? {
    prompt: existing ? String(existing.prompt_price_per_1m) : "",
    completion: existing ? String(existing.completion_price_per_1m) : "",
    cache: existing ? String(existing.cache_price_per_1m) : "",
  }
}

export function nextCostRateDrafts(
  model: string,
  pricingMap: Map<string, PricingEntry>,
  drafts: CostRateDrafts,
  field: keyof CostRateDraft,
  value: string,
): CostRateDrafts {
  return {
    ...drafts,
    [model]: { ...getCostRateDraft(model, pricingMap, drafts), [field]: value },
  }
}

export function validateCostRateDraft(draft: CostRateDraft): CostRateValidation {
  if ([draft.prompt, draft.completion, draft.cache].some((value) => value.trim() === "")) {
    return { valid: false, reason: "missing" }
  }
  const prices = [draft.prompt, draft.completion, draft.cache].map(Number) as [number, number, number]
  if (prices.some((price) => !Number.isFinite(price) || price < 0)) {
    return { valid: false, reason: "invalid" }
  }
  return { valid: true, prices }
}

export function buildCostRateSavePayload(model: string, prices: [number, number, number]): PricingEntry {
  return {
    model,
    prompt_price_per_1m: prices[0],
    completion_price_per_1m: prices[1],
    cache_price_per_1m: prices[2],
  }
}

export function buildCostRateSaveCommand(model: string, draft: CostRateDraft): CostRateSaveCommand {
  const validation = validateCostRateDraft(draft)
  if (!validation.valid) {
    return validation
  }
  return { valid: true, payload: buildCostRateSavePayload(model, validation.prices) }
}
