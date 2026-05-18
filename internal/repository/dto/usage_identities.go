package dto

import "time"

// APIKeyAliasTargetRecord 是 raw API key alias 管理面的聚合目标。
type APIKeyAliasTargetRecord struct {
	Identity             string
	Provider             string
	RequestCount         int64
	SuccessCount         int64
	FailureCount         int64
	InputTokens          int64
	OutputTokens         int64
	ReasoningTokens      int64
	CachedTokens         int64
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
	FirstUsedAt          *time.Time
	LastUsedAt           *time.Time
}

// UsageIdentityStatsDelta 是 usage identity 聚合统计的仓储层扫描结果。
type UsageIdentityStatsDelta struct {
	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
	FirstUsedAt     *time.Time
	LastUsedAt      *time.Time
	MaxUsageEventID uint
}
