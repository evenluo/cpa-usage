export type TimeGranularity = "hour" | "day"
export type TimeRange = "today" | "yesterday" | "24h" | "7d" | "30d"
export type CostStatus = "available" | "partial" | "unavailable"
export type CacheReadShareState = "available" | "no_cache_data" | "no_prompt_input"

export interface AnalyticsSummary {
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  input_tokens: number
  cached_tokens: number
  success_rate: number
  cost_available: boolean
  cost_status: CostStatus
  cache_read_share: number
  cache_read_share_state: CacheReadShareState
  estimated_cache_savings?: number
}

export interface TrendPoint {
  label: string
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  cost_available: boolean
  cost_status: CostStatus
}

export interface KeyAliasBreakdown {
  label: string
  alias: string
  traceability: string
  identity: string
  auth_type: number
  auth_type_name: string
  type: string
  provider: string
  is_deleted: boolean
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  success_rate: number
  last_used_at: string | null
  cost_available: boolean
  cost_status: CostStatus
  trend: Array<Pick<TrendPoint, "label" | "total_cost" | "total_tokens" | "cost_available" | "cost_status">>
}

export interface ModelDistribution {
  model: string
  provider: string
  total_cost: number
  total_tokens: number
  input_tokens: number
  cached_tokens: number
  cache_read_share: number
  cache_read_share_state: CacheReadShareState
  estimated_cache_savings?: number
  request_count: number
  success_count: number
  failure_count: number
  success_rate: number
  total_latency_ms: number
  latency_sample_count: number
  average_latency_ms: number
  cost_available: boolean
  cost_status: CostStatus
}

export interface Insight {
  type: string
  severity: "green" | "blue" | "violet" | "amber"
  title: string
  detail: string
  subject: string
  metric_label: string
  metric_value: number
  count: number
  cost_status: CostStatus
}

export interface ProviderOption {
  provider: string
  request_count: number
  total_tokens: number
  total_cost: number
  cost_available: boolean
  cost_status: CostStatus
}

export interface Comparison {
  has_previous_period: boolean
  total_cost_change_pct?: number | null
  total_tokens_change_pct?: number | null
  request_count_change_pct?: number | null
  success_rate_change_pp?: number | null
}

export interface HeatmapCell {
  hour: number
  in_range: boolean
  bucket_start: string
  bucket_end: string
  total_tokens: number
  total_cost: number
  request_count: number
  failure_count: number
  cost_available: boolean
  cost_status: CostStatus
}

export interface HeatmapRow {
  date: string
  label: string
  cells: HeatmapCell[]
}

export interface HeatmapData {
  measure: "tokens"
  max_tokens: number
  max_cost: number
  max_requests: number
  max_failures: number
  rows: HeatmapRow[]
}

export interface AnalyticsResponse {
  granularity?: TimeGranularity
  summary: AnalyticsSummary
  comparison?: Comparison
  heatmap?: HeatmapData
  trend: TrendPoint[]
  key_alias_breakdown?: KeyAliasBreakdown[]
  model_distribution?: ModelDistribution[]
  time_breakdown?: TrendPoint[]
  insights?: Insight[]
  provider_options?: ProviderOption[]
}

export interface KeyIdentity {
  id: number
  name: string
  displayName: string
  alias: string
  auth_type: number
  auth_type_name: string
  identity: string
  type: string
  provider: string
  total_tokens: number
  total_cost: number
  cost_available: boolean
  last_used_at: string | null
}

export interface KeyIdentityPage {
  identities: KeyIdentity[]
  total_pages?: number
}

export interface UsageEvent {
  id?: number
  timestamp: string
  model: string
  source: string
  auth_index?: string
  failed: boolean
  latency_ms: number
  tokens: { total_tokens: number }
}

export interface UsageEventsPage {
  events: UsageEvent[]
}

export interface PricingEntry {
  model: string
  prompt_price_per_1m: number
  completion_price_per_1m: number
  cache_price_per_1m: number
}

export interface PricingPayload {
  pricing: PricingEntry[]
}

export interface UsedModelsPayload {
  models: string[]
}

export interface StatusPayload {
  running?: boolean
  sync_running?: boolean
  last_status?: string
  last_run_at?: string
  last_error?: string
  last_warning?: string
  timezone?: string
  version?: string
  updateCheckEnabled?: boolean
}

export interface AuthSessionPayload {
  authenticated?: boolean
}
