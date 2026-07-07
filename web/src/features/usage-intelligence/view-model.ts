import type {
  AnalyticsCoreResponse,
  AnalyticsResponse,
  CostStatus,
  HeatmapData,
  KeyAliasBreakdown,
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
  fixedHeatmap?: HeatmapData
  serviceHealth?: ServiceHealth
  hasLeaderboardBreakdown: boolean
  leaderboardSortLabel: string
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

export function buildUsageDashboardViewModel(input: {
  analytics?: AnalyticsResponse | AnalyticsCoreResponse
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
  return {
    trend,
    keyAliases,
    apiKeys,
    leaderboardRows: getLeaderboardRows(input.leaderboardScope, apiKeys, keyAliases),
    providerOptions: input.analytics?.provider_options ?? [],
    fixedHeatmap: input.fixedHeatmap,
    serviceHealth: input.requestHealth?.service_health,
    hasLeaderboardBreakdown,
    leaderboardSortLabel: getLeaderboardSortLabel(input.analytics?.summary?.cost_status),
    kpiData: deriveKpiSparklineData(trend),
  }
}
