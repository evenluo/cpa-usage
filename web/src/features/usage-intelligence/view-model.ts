import type {
  AnalyticsCoreResponse,
  CacheReadShareState,
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
  tokens: number[]
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
    tokens: trend.map((point) => point.total_tokens),
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
  if (costStatus === "partial") return "Sort: Cost partial"
  return "Sort: Cost"
}

export function getCacheReadShareCaption(state?: CacheReadShareState, coverage?: number): string | undefined {
  if (state === undefined) return undefined
  if (state === "no_prompt_input") return "No prompt input"
  if (state === "no_cache_data") return "No exact cache data"
  const label = state === "available" ? "Exact" : "Partial"
  return `${label} · covers ${(coverage ?? 0).toFixed(1)}% of prompt input`
}

export function getCacheReadShareValue(value?: number, state?: CacheReadShareState): number | undefined {
  if (state !== "available" && state !== "partial") return undefined
  return value
}

export function getModelMixPresentation(costStatus?: CostStatus): {
  measure: "cost" | "tokens"
  costStateLabel: string
} {
  if (costStatus === "available") {
    return { measure: "cost", costStateLabel: "Cost state: available · Cost share" }
  }
  if (costStatus === "partial") {
    return { measure: "tokens", costStateLabel: "Cost state: partial · Token share" }
  }
  return { measure: "tokens", costStateLabel: "Cost state: unavailable · Token share" }
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
  // The Cache KPI is the current compact presentation owner; keep the parallel
  // cache insight hidden so partial/exact state is not duplicated on the page.
  const insights = (input.analytics?.insights ?? []).filter((insight) => insight.type !== "cache_efficiency")
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
    kpiData: deriveKpiSparklineData(trend),
  }
}
