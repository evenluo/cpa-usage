export type TimeGranularity = "hour" | "day"
export type TimeRange = "today" | "yesterday" | "24h" | "7d" | "30d"
export type CostStatus = "available" | "partial" | "unavailable"
export type CacheReadShareState = "available" | "partial" | "no_cache_data" | "no_prompt_input"

export type AccountingState = "absent" | "malformed" | "unsupported_accounting_version" | "unsupported_schema_version" | "missing" | "unknown_quality" | "invalid" | "valid"
export type AccountingQuality = "complete" | "inconsistent" | "unclassified"

export interface CanonicalComposition<T = number> {
  total_tokens: T
  input: { total_tokens: T; uncached_tokens: T; cache_read_tokens: T; cache_write_tokens: T }
  output: { total_tokens: T; non_reasoning_tokens: T; reasoning_tokens: T }
  unclassified_tokens: T
}

export interface AccountingSummary {
  total_attempts: number
  valid_attempts: number
  coverage_pct: number | null
  states: Record<AccountingState, number>
  valid_quality: Record<AccountingQuality, number>
  composition: CanonicalComposition
}

export interface UsageAttemptFacts {
  generate: boolean | null
  stream: boolean | null
  request_service_tier: string | null
  response_service_tier: string | null
  output_tps: number | null
  accounting: CanonicalComposition<number | null> & {
    state: AccountingState
    accounting_version: number | null
    schema_version: number | null
    quality: AccountingQuality | "unknown" | null
  }
}

export interface AnalyticsSummary {
  accounting?: AccountingSummary
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cached_tokens: number
  cache_read_tokens: number
  success_rate: number
  cost_available: boolean
  cost_status: CostStatus
  cache_read_share: number
  cache_read_coverage: number
  cache_read_share_state: CacheReadShareState
  estimated_cache_savings?: number
}

export interface TrendPoint {
  label: string
  total_cost: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cached_tokens: number
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
  output_tokens: number
  reasoning_tokens: number
  cached_tokens: number
  cache_read_tokens: number
  cache_read_share: number
  cache_read_coverage: number
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

export interface AnalyticsCoreResponse {
  granularity?: TimeGranularity
  summary: AnalyticsSummary
  trend: TrendPoint[]
  key_alias_breakdown?: KeyAliasBreakdown[]
  api_key_breakdown?: KeyAliasBreakdown[]
  model_distribution?: ModelDistribution[]
  insights?: Insight[]
  provider_options?: ProviderOption[]
}

export interface AnalyticsHeatmapResponse {
  granularity?: TimeGranularity
  heatmap: HeatmapData
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
  disabled: boolean
  status?: "active" | "pending" | "refreshing" | "error" | "disabled" | "unknown" | "other" | null
  unavailable?: boolean | null
  last_refresh?: string | null
  next_retry_after?: string | null
  metadata_observed_at?: string | null
  plan_type?: string | null
  active_start?: string | null
  active_until?: string | null
  total_tokens: number
  total_cost: number
  cost_available: boolean
  last_used_at: string | null
}

export interface APIKeyAliasTarget {
  id: string
  identity: string
  displayName: string
  alias: string
  provider: string
  auth_type: number
  auth_type_name: string
  total_requests: number
  success_count: number
  failure_count: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cached_tokens: number
  total_tokens: number
  total_cost: number
  cost_available: boolean
  cost_status: CostStatus
  first_used_at: string | null
  last_used_at: string | null
}

export interface KeyIdentityPage {
  identities: KeyIdentity[]
  total_count: number
  page: number
  page_size: number
  total_pages: number
}

export interface QuotaWindow {
  duration?: number
  unit?: string
  seconds?: number
}

export interface QuotaRow {
  key: string
  label?: string
  scope?: string
  metric?: string
  planType?: string
  used?: number
  limit?: number
  remaining?: number
  usedPercent?: number
  remainingFraction?: number
  allowed?: boolean
  limitReached?: boolean
  window?: QuotaWindow
  resetAt?: string
  resetAfterSeconds?: number
}

export interface QuotaCheckResponse {
  id: string
  quota: QuotaRow[]
}

export interface QuotaCacheItem extends QuotaCheckResponse {
  cachedAt?: string
  expiresAt?: string
}

export interface QuotaCacheResponse {
  items: QuotaCacheItem[]
}

export interface QuotaRefreshTaskID {
  authIndex: string
  taskId: string
}

export interface QuotaRefreshRejectedAuthIndex {
  authIndex: string
  error: string
}

export interface QuotaRefreshResponse {
  tasks: QuotaRefreshTaskID[]
  rejected: QuotaRefreshRejectedAuthIndex[]
  accepted: number
  skipped: number
  limit: number
}

export type QuotaRefreshTaskStatus = "queued" | "running" | "completed" | "failed"

export interface QuotaRefreshTaskResponse {
  taskId: string
  authIndex: string
  status: QuotaRefreshTaskStatus
  quota?: QuotaCheckResponse
  error?: string
  cachedAt?: string
  expiresAt?: string
}

export interface APIKeyAliasTargetPage {
  api_keys: APIKeyAliasTarget[]
  total_count: number
  page: number
  page_size: number
  total_pages: number
}

export interface UsageEvent {
  attempt_facts?: UsageAttemptFacts
  id?: number
  timestamp: string
  model: string
  model_alias?: string
  endpoint?: string
  request_id?: string
  status_code?: number
  executor_type?: string
  reasoning_effort?: string
  service_tier?: string
  source: string
  auth_index?: string
  api_key_alias?: string
  api_key_display?: string
  failed: boolean
  latency_ms: number
  ttft_ms: number | null
  output_tps: number | null
  tokens: {
    input_tokens?: number
    output_tokens: number
    reasoning_tokens?: number
    cached_tokens?: number
    cache_read_tokens?: number
    cache_creation_tokens?: number
    total_tokens: number
  }
}

export interface UsageEventsPage {
  events: UsageEvent[]
  window_end?: string
  total_count: number
  page: number
  page_size: number
  total_pages: number
}

export interface UsageDiagnosticSelection {
  provider?: string
  model?: string
  modelAlias?: string
  account?: string
  endpoint?: string
  status?: string
  requestId?: string
  windowEnd?: string
}

export interface UsageModelMapping {
  model_alias: string
  model: string
  provider: string
  attempt_count: number
  failure_count: number
  failure_share: number
  latency_sample_count: number
  mean_latency_ms: number
  total_cost: number
  cost_available: boolean
  cost_status: CostStatus
}

export interface UsageModelMappingDistribution {
  window_start: string
  window_end: string
  total_attempts: number
  observed_alias_attempts: number
  missing_alias_attempts: number
  alias_coverage: number
  observed_total_cost: number
  observed_cost_available: boolean
  observed_cost_status: CostStatus
  mappings: UsageModelMapping[]
  other_attempts: number
}

export interface UsageFailureBreakdownItem {
  value: string
  label: string
  category?: string
  count: number
}

export interface UsageFailureBreakdown {
  items: UsageFailureBreakdownItem[]
  other_count: number
}

export interface UsageFailureDistribution {
  window_start: string
  window_end: string
  total_failures: number
  categories: UsageFailureBreakdown
  statuses: UsageFailureBreakdown
  providers: UsageFailureBreakdown
  accounts: UsageFailureBreakdown
  models: UsageFailureBreakdown
  endpoints: UsageFailureBreakdown
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
  rollup_backfill?: {
    status: string
    target_bucket_start?: string
    covered_bucket_start?: string
    started_at?: string
    completed_at?: string
    failed_at?: string
    last_error?: string
  }
}

export interface MetricsPayload {
  uptime_seconds?: number
  poller_running?: boolean
  poller_sync_running?: boolean
  redis_inbox_pending?: number
  redis_events_processed_total?: number
  redis_events_processed_batches_total?: number
  redis_events_last_processed_at?: string
  redis_events_processing_rate_per_minute?: number
  db_unavailable?: boolean
}

export interface AuthSessionPayload {
  authenticated: boolean
}

export interface ServiceHealthBlock {
  start_time: string
  end_time: string
  success: number
  failure: number
  rate: number
}

export interface ServiceHealth {
  total_success: number
  total_failure: number
  success_rate: number
  rows: number
  columns: number
  bucket_seconds: number
  window_start: string
  window_end: string
  block_details: ServiceHealthBlock[]
}

export interface UsageOverviewResponse {
  service_health: ServiceHealth
}

export interface RequestHealthResponse {
  service_health: ServiceHealth
}
