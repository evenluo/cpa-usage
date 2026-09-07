import type {
  AccountingState,
  AccountingSummary,
  AnalyticsCoreResponse,
  CacheReadShareState,
  CanonicalComposition,
  CostStatus,
  HeatmapData,
  Insight,
  KeyAliasBreakdown,
  ModelDistribution,
  ProviderOption,
  RequestHealthResponse,
  ServiceHealth,
  TimeGranularity,
  TimeRange,
  TrendPoint,
} from "@/types/api"

export const TIME_RANGES: { value: TimeRange; label: string }[] = [
  { value: "today", label: "Today" },
  { value: "yesterday", label: "Yesterday" },
  { value: "24h", label: "Last 24h" },
  { value: "7d", label: "7 days" },
  { value: "30d", label: "30 days" },
]

export const DEFAULT_TIME_RANGE: TimeRange = "7d"
export const SELECTED_TIME_RANGE_STORAGE_KEY = "cpa-usage:selected-time-range"

export type LeaderboardScope = "account" | "api-key"

export type TrendView = "cost-token" | "requests-token" | "tokens"

export interface UsageKpiSparklineData {
  cost: Array<number | null>
  tokens: Array<number | null>
  requests: number[]
  successRate: number[]
}

export interface UsageDashboardViewModel {
  trend: TrendPoint[]
  keyAliases: KeyAliasBreakdown[]
  apiKeys: KeyAliasBreakdown[]
  leaderboardRows: KeyAliasBreakdown[]
  providerOptions: ProviderOption[]
  modelDistribution: ModelDistribution[]
  insights: Insight[]
  hasModelDistribution: boolean
  hasInsights: boolean
  modelMixMeasure: "cost" | "tokens"
  modelMixCostStateLabel: string
  fixedHeatmap?: HeatmapData
  serviceHealth?: ServiceHealth
  hasLeaderboardBreakdown: boolean
  leaderboardSortLabel: string
  cacheReadShareCaption?: string
  cacheReadShareValue?: number
  accountingCaption: string
  kpiData: UsageKpiSparklineData | null
}

export function getDefaultGranularity(range: TimeRange): TimeGranularity {
  if (range === "30d") return "day"
  return "hour"
}

export function isTimeRange(value: string | null): value is TimeRange {
  return TIME_RANGES.some((range) => range.value === value)
}

export function resolveStoredTimeRange(value: string | null): TimeRange {
  return isTimeRange(value) ? value : DEFAULT_TIME_RANGE
}

export function getEffectiveGranularity(
  range: TimeRange,
  selectedGranularity: TimeGranularity | null,
): TimeGranularity {
  return selectedGranularity ?? getDefaultGranularity(range)
}

export function deriveKpiSparklineData(trend: TrendPoint[]): UsageKpiSparklineData | null {
  if (trend.length === 0) return null
  return {
    cost: trend.map((point) => (point.cost_status === "unavailable" ? null : point.total_cost)),
    tokens: trend.map((point) => point.canonical_valid_attempts > 0 ? point.total_tokens : null),
    requests: trend.map((point) => point.request_count),
    successRate: trend.map((point) => {
      const success = Math.max(point.request_count - point.failure_count, 0)
      return point.request_count > 0 ? (success / point.request_count) * 100 : 0
    }),
  }
}

export function getLeaderboardRows(
  scope: LeaderboardScope,
  apiKeys: KeyAliasBreakdown[],
  keyAliases: KeyAliasBreakdown[],
): KeyAliasBreakdown[] {
  return scope === "api-key" ? apiKeys : keyAliases
}

export function getLeaderboardSortLabel(costStatus?: CostStatus): string {
  if (costStatus === "unavailable") return "Sort: Tokens"
  if (costStatus === "partial") return "Sort: Local estimate incomplete"
  return "Sort: Cost"
}

export function getCacheReadShareCaption(state?: CacheReadShareState, coverage?: number): string | undefined {
  if (state === undefined) return undefined
  if (state === "no_prompt_input") return "No canonical input"
  if (state === "no_cache_data") return "No exact cache data"
  const label = state === "available" ? "Exact" : "Partial"
  if (coverage === undefined) return label
  return `${label} · covers ${coverage.toFixed(1)}% of canonical input`
}

export function getCacheReadShareValue(value?: number, state?: CacheReadShareState): number | undefined {
  if (state !== "available" && state !== "partial") return undefined
  return value
}

export const ACCOUNTING_STATE_LABELS: Record<AccountingState, string> = {
  absent: "Canonical facts absent",
  valid: "Valid structure",
}

export function getAccountingCaption(accounting: AccountingSummary): string {
  if (accounting.coverage_pct === null) return "No attempts"
  if (accounting.valid_attempts === 0) return "Canonical tokens unavailable"
  const quality = accounting.valid_quality.complete === accounting.valid_attempts ? "complete quality" : "qualified quality"
  return `Canonical: ${accounting.coverage_pct.toFixed(1)}% of attempts · ${quality}`
}

// Render the repository's disjoint canonical buckets without adding a subset
// such as reasoning or cache tokens back into its parent total.
export function getCanonicalTokenFields(composition: CanonicalComposition<number | null>): Array<[string, number | null]> {
  return [
    ["Canonical total", composition.total_tokens],
    ["Canonical input total", composition.input.total_tokens],
    ["Uncached input", composition.input.uncached_tokens],
    ["Cache read input", composition.input.cache_read_tokens],
    ["Cache write input", composition.input.cache_write_tokens],
    ["Canonical output total", composition.output.total_tokens],
    ["Non-reasoning output", composition.output.non_reasoning_tokens],
    ["Reasoning output", composition.output.reasoning_tokens],
    ["Unclassified tokens", composition.unclassified_tokens],
  ]
}

export function getModelMixPresentation(costStatus?: CostStatus): {
  measure: "cost" | "tokens"
  costStateLabel: string
} {
  if (costStatus === "available") {
    return { measure: "cost", costStateLabel: "By cost" }
  }
  if (costStatus === "partial") {
    return { measure: "tokens", costStateLabel: "Local estimate incomplete, by tokens" }
  }
  return { measure: "tokens", costStateLabel: "Cost unavailable, by tokens" }
}

// The Attention rail surfaces warning-level signals only; "amber" is the
// warning tier in the backend insight severity taxonomy
// (green/blue/violet/amber, see internal/repository/analytics_insights.go).
function isAttentionSignal(insight: Insight): boolean {
  return insight.severity === "amber"
}

export function buildUsageDashboardViewModel(input: {
  analytics?: AnalyticsCoreResponse
  fixedHeatmap?: HeatmapData
  requestHealth?: RequestHealthResponse
  leaderboardScope: LeaderboardScope
}): UsageDashboardViewModel {
  const trend = input.analytics?.trend ?? []
  const keyAliases = input.analytics?.key_alias_breakdown ?? []
  const apiKeys = input.analytics?.api_key_breakdown ?? []
  const hasLeaderboardBreakdown = input.leaderboardScope === "api-key"
    ? Array.isArray(input.analytics?.api_key_breakdown)
    : Array.isArray(input.analytics?.key_alias_breakdown)
  const modelDistribution = input.analytics?.model_distribution ?? []
  // The Cache KPI is the compact cache presentation owner; keep its parallel
  // insight hidden and reserve the Attention rail for warning-level signals.
  const insights = (input.analytics?.insights ?? []).filter(
    (insight) => insight.type !== "cache_efficiency" && isAttentionSignal(insight),
  )
  const modelMix = getModelMixPresentation(input.analytics?.summary?.cost_status)
  return {
    trend,
    keyAliases,
    apiKeys,
    leaderboardRows: getLeaderboardRows(input.leaderboardScope, apiKeys, keyAliases),
    providerOptions: input.analytics?.provider_options ?? [],
    modelDistribution,
    insights,
    hasModelDistribution: Array.isArray(input.analytics?.model_distribution),
    hasInsights: Array.isArray(input.analytics?.insights),
    modelMixMeasure: modelMix.measure,
    modelMixCostStateLabel: modelMix.costStateLabel,
    fixedHeatmap: input.fixedHeatmap,
    serviceHealth: input.requestHealth?.service_health,
    hasLeaderboardBreakdown,
    leaderboardSortLabel: getLeaderboardSortLabel(input.analytics?.summary?.cost_status),
    cacheReadShareCaption: getCacheReadShareCaption(input.analytics?.summary?.cache_read_share_state, input.analytics?.summary?.cache_read_coverage),
    cacheReadShareValue: getCacheReadShareValue(input.analytics?.summary?.cache_read_share, input.analytics?.summary?.cache_read_share_state),
    accountingCaption: input.analytics
      ? getAccountingCaption(input.analytics.summary.accounting)
      : "Canonical tokens unavailable",
    kpiData: deriveKpiSparklineData(trend),
  }
}
