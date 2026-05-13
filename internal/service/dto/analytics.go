package dto

import "time"

type AnalyticsSummary struct {
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

type AnalyticsTrendPoint struct {
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

type AnalyticsKeyAliasTrendPoint struct {
	Label         string
	TotalCost     float64
	TotalTokens   int64
	CostAvailable bool
	CostStatus    string
}

type AnalyticsKeyAliasBreakdown struct {
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
	Trend         []AnalyticsKeyAliasTrendPoint
}

type AnalyticsModelBreakdown struct {
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

type AnalyticsInsight struct {
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

type AnalyticsSummarySnapshot struct {
	Summary           AnalyticsSummary
	Trend             []AnalyticsTrendPoint
	KeyAliasBreakdown []AnalyticsKeyAliasBreakdown
	ModelBreakdown    []AnalyticsModelBreakdown
	TimeBreakdown     []AnalyticsTrendPoint
	Insights          []AnalyticsInsight
}
