import { describe, expect, it } from "vitest"
import type { APIKeyAliasTarget, KeyIdentity, PricingEntry } from "@/types/api"
import {
  buildCostRateModels,
  buildCostRateSavePayload,
  buildPricingMap,
  canSaveKeyAlias,
  countAliasedRows,
  countMissingCostRates,
  filterKeyAliasRows,
  getCostRateDraft,
  normalizeAccountKeyRows,
  normalizeAPIKeyRows,
  validateCostRateDraft,
} from "./model"

const apiKey: APIKeyAliasTarget = {
  id: "sk-alpha",
  identity: "sk-alpha-trace",
  displayName: "sk-alpha…trace",
  alias: "Alpha Alias",
  provider: "OpenAI",
  auth_type: 2,
  auth_type_name: "apikey",
  total_requests: 3,
  success_count: 2,
  failure_count: 1,
  input_tokens: 10,
  output_tokens: 5,
  reasoning_tokens: 0,
  cached_tokens: 2,
  total_tokens: 17,
  total_cost: 1.5,
  cost_available: true,
  cost_status: "available",
  first_used_at: null,
  last_used_at: null,
}

const accountKey: KeyIdentity = {
  id: 7,
  name: "Account Name",
  displayName: "Account Display",
  alias: "Account Alias",
  auth_type: 1,
  auth_type_name: "oauth",
  identity: "acct-trace",
  type: "claude",
  provider: "Anthropic",
  total_tokens: 25,
  total_cost: 2,
  cost_available: false,
  last_used_at: "2026-05-18T00:00:00Z",
}

const configuredPricing: PricingEntry = {
  model: "configured-model",
  prompt_price_per_1m: 1,
  completion_price_per_1m: 2,
  cache_price_per_1m: 0.5,
}

describe("Reference Data feature model", () => {
  it("normalizes API Key and account rows for Key Alias management", () => {
    expect(normalizeAPIKeyRows([apiKey])).toEqual([{
      id: "sk-alpha",
      alias: "Alpha Alias",
      displayName: "sk-alpha…trace",
      name: "",
      identity: "sk-alpha-trace",
      provider: "OpenAI",
      type: "api-key",
      auth_type_name: "apikey",
      total_tokens: 17,
      total_cost: 1.5,
      cost_available: true,
      last_used_at: null,
    }])
    expect(normalizeAccountKeyRows([accountKey])[0]).toMatchObject({
      id: "7",
      alias: "Account Alias",
      name: "Account Name",
      identity: "acct-trace",
      provider: "Anthropic",
      type: "claude",
      auth_type_name: "oauth",
    })
  })

  it("searches Key Alias rows across alias, display, CPA Key traceability, provider, type, and auth type", () => {
    const rows = [...normalizeAPIKeyRows([apiKey]), ...normalizeAccountKeyRows([accountKey])]

    expect(filterKeyAliasRows(rows, "alpha alias")).toHaveLength(1)
    expect(filterKeyAliasRows(rows, "trace")).toHaveLength(2)
    expect(filterKeyAliasRows(rows, "anthropic")).toEqual([rows[1]])
    expect(filterKeyAliasRows(rows, "claude")).toEqual([rows[1]])
    expect(filterKeyAliasRows(rows, "oauth")).toEqual([rows[1]])
  })

  it("rejects empty Key Alias saves while keeping clear as a separate action", () => {
    expect(canSaveKeyAlias("  ")).toBe(false)
    expect(canSaveKeyAlias("Alpha Ops")).toBe(true)
    expect(countAliasedRows([{ alias: "Alpha" }, { alias: "" }])).toBe(1)
  })

  it("prioritizes missing Cost Rates and creates drafts from configured rates", () => {
    const models = buildCostRateModels(["missing-model", "configured-model"], [configuredPricing])
    const pricingMap = buildPricingMap([configuredPricing])

    expect(models).toEqual(["missing-model", "configured-model"])
    expect(countMissingCostRates(models, pricingMap)).toBe(1)
    expect(getCostRateDraft("configured-model", pricingMap, {})).toEqual({ prompt: "1", completion: "2", cache: "0.5" })
    expect(getCostRateDraft("missing-model", pricingMap, {})).toEqual({ prompt: "", completion: "", cache: "" })
    expect(getCostRateDraft("configured-model", pricingMap, { "configured-model": { prompt: "3", completion: "4", cache: "5" } })).toEqual({ prompt: "3", completion: "4", cache: "5" })
  })

  it("requires complete non-negative Cost Rate drafts before building save payloads", () => {
    expect(validateCostRateDraft({ prompt: "", completion: "1", cache: "0" })).toEqual({ valid: false, reason: "missing" })
    expect(validateCostRateDraft({ prompt: "1", completion: "-1", cache: "0" })).toEqual({ valid: false, reason: "invalid" })

    const result = validateCostRateDraft({ prompt: "1", completion: "2.5", cache: "0" })
    expect(result).toEqual({ valid: true, prices: [1, 2.5, 0] })
    if (result.valid) {
      expect(buildCostRateSavePayload("configured-model", result.prices)).toEqual({
        model: "configured-model",
        prompt_price_per_1m: 1,
        completion_price_per_1m: 2.5,
        cache_price_per_1m: 0,
      })
    }
  })
})
