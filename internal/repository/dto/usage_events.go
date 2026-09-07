package dto

import "time"

// UsageEventsPageRecord 是 usage events 列表的仓储查询结果。
type UsageEventsPageRecord struct {
	Events     []UsageEventRecord
	Models     []string
	TotalCount int64
	Page       int
	PageSize   int
	TotalPages int
}

// UsageEventFilterOptionsRecord 是 usage events 筛选项的仓储查询结果。
type UsageEventFilterOptionsRecord struct {
	Models []string
}

// UsageEventRecord 是单条 usage event 的查询结果。
type UsageEventRecord struct {
	AttemptFacts        UsageAttemptFacts
	ID                  uint
	Timestamp           time.Time
	APIGroupKey         string
	APIKeyIdentity      string
	Model               string
	ModelAlias          string
	Endpoint            string
	RequestID           string
	AuthType            string
	Provider            string
	Source              string
	AuthIndex           string
	Failed              bool
	StatusCode          *int
	ExecutorType        string
	ReasoningEffort     string
	ServiceTier         string
	LatencyMS           int64
	TTFTMS              *int64
	OutputTPS           *float64
	InputTokens         int64
	OutputTokens        int64
	ReasoningTokens     int64
	CachedTokens        int64
	CacheReadTokens     *int64
	CacheCreationTokens *int64
	TotalTokens         int64
}

const UsageFailureBreakdownLimit = 8

// UsageFailureDistributionRecord keeps each breakdown independently bounded
// while OtherCount preserves parity with TotalFailures.
type UsageFailureDistributionRecord struct {
	TotalFailures int64
	Categories    UsageFailureBreakdownRecord
	Statuses      UsageFailureBreakdownRecord
	Providers     UsageFailureBreakdownRecord
	Accounts      UsageFailureBreakdownRecord
	Models        UsageFailureBreakdownRecord
	Endpoints     UsageFailureBreakdownRecord
}

type UsageFailureBreakdownRecord struct {
	Items      []UsageFailureBreakdownItemRecord
	OtherCount int64
}

type UsageFailureBreakdownItemRecord struct {
	Value string
	Count int64
}

const UsageModelMappingLimit = 20

// UsageModelMappingDistributionRecord describes the fixed-window population
// that has an observed CPA alias label and its bounded actual-model mappings.
type UsageModelMappingDistributionRecord struct {
	TotalAttempts          int64
	ObservedAliasAttempts  int64
	CanonicalValidAttempts int64
	MissingAliasAttempts   int64
	ObservedTotalCost      float64
	ObservedCostAvailable  bool
	ObservedCostStatus     string
	Mappings               []UsageModelMappingRecord
	OtherAttempts          int64
}

type UsageModelMappingRecord struct {
	ModelAlias             string
	Model                  string
	Provider               string
	AttemptCount           int64
	FailureCount           int64
	CanonicalValidAttempts int64
	FailureShare           float64
	TotalLatencyMS         int64
	LatencySampleCount     int64
	MeanLatencyMS          float64
	TotalCost              float64
	CostAvailable          bool
	CostStatus             string
}
