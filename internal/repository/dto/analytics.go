package dto

import "time"

const (
	AnalyticsCostStatusAvailable   = "available"
	AnalyticsCostStatusPartial     = "partial"
	AnalyticsCostStatusUnavailable = "unavailable"

	AnalyticsCacheReadShareStateAvailable     = "available"
	AnalyticsCacheReadShareStateNoCacheData   = "no_cache_data"
	AnalyticsCacheReadShareStateNoPromptInput = "no_prompt_input"
)

type AnalyticsSummaryRecord struct {
	TotalCost             float64
	TotalTokens           int64
	RequestCount          int64
	SuccessCount          int64
	FailureCount          int64
	InputTokens           int64
	CachedTokens          int64
	ReasoningTokens       int64
	SuccessRate           float64
	CostAvailable         bool
	CostStatus            string
	CacheReadShare        float64
	CacheReadShareState   string
	EstimatedCacheSavings *float64
}

type AnalyticsTrendPointRecord struct {
	Label         string
	BucketStart   time.Time
	BucketEnd     time.Time
	TotalCost     float64
	TotalTokens   int64
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	CostAvailable bool
	CostStatus    string
}

type AnalyticsKeyAliasTrendPointRecord struct {
	Label         string
	TotalCost     float64
	TotalTokens   int64
	CostAvailable bool
	CostStatus    string
}

type AnalyticsKeyAliasBreakdownRecord struct {
	AuthType      int
	Identity      string
	Alias         string
	Name          string
	AuthTypeName  string
	Type          string
	Provider      string
	Prefix        string
	BaseURL       string
	IsDeleted     bool
	TotalCost     float64
	TotalTokens   int64
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	SuccessRate   float64
	LastUsedAt    *time.Time
	CostAvailable bool
	CostStatus    string
	Trend         []AnalyticsKeyAliasTrendPointRecord
}

type AnalyticsModelBreakdownRecord struct {
	Model                 string
	Provider              string
	TotalCost             float64
	TotalTokens           int64
	RequestCount          int64
	SuccessCount          int64
	FailureCount          int64
	InputTokens           int64
	CachedTokens          int64
	SuccessRate           float64
	TotalLatencyMS        int64
	LatencySampleCount    int64
	AverageLatencyMS      float64
	CostAvailable         bool
	CostStatus            string
	CacheReadShare        float64
	CacheReadShareState   string
	EstimatedCacheSavings *float64
}

type AnalyticsInsightRecord struct {
	Type        string
	Severity    string
	Title       string
	Detail      string
	Subject     string
	MetricLabel string
	MetricValue float64
	Count       int64
	CostStatus  string
}

type AnalyticsProviderOptionRecord struct {
	Provider      string
	RequestCount  int64
	TotalTokens   int64
	TotalCost     float64
	CostAvailable bool
	CostStatus    string
}

type AnalyticsComparisonRecord struct {
	HasPreviousPeriod     bool
	TotalCostChangePct    *float64
	TotalTokensChangePct  *float64
	RequestCountChangePct *float64
	SuccessRateChangePP   *float64
}

type AnalyticsSummarySnapshot struct {
	Summary            AnalyticsSummaryRecord
	Trend              []AnalyticsTrendPointRecord
	KeyAliasBreakdown  []AnalyticsKeyAliasBreakdownRecord
	ModelBreakdown     []AnalyticsModelBreakdownRecord
	TimeBreakdown      []AnalyticsTrendPointRecord
	Insights           []AnalyticsInsightRecord
	ProviderOptions    []AnalyticsProviderOptionRecord
	PreviousRangeStart *time.Time
	PreviousRangeEnd   *time.Time
	Comparison         AnalyticsComparisonRecord
}
