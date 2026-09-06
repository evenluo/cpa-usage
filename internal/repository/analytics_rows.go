package repository

type analyticsAggregateRow struct {
	Bucket                               string
	RequestCount                         int64
	SuccessCount                         int64
	FailureCount                         int64
	InputTokens                          int64
	OutputTokens                         int64
	TotalTokens                          int64
	TotalCost                            float64
	CachedTokens                         int64
	CacheReadTokens                      int64
	CacheReadObservedInputTokens         int64
	ReasoningTokens                      int64
	CacheSavings                         float64
	CacheSavingsEligibleRows             int64
	CacheSavingsIneligibleRows           int64
	MissingPricingEvents                 int64
	PricedBillableEvents                 int64
	AccountingAbsentAttempts             int64
	AccountingMalformedAttempts          int64
	AccountingUnsupportedVersionAttempts int64
	AccountingUnsupportedSchemaAttempts  int64
	AccountingMissingAttempts            int64
	AccountingUnknownQualityAttempts     int64
	AccountingInvalidAttempts            int64
	AccountingValidAttempts              int64
	AccountingValidCompleteAttempts      int64
	AccountingValidInconsistentAttempts  int64
	AccountingValidUnclassifiedAttempts  int64
	CanonicalTotalTokens                 int64
	CanonicalInputTokens                 int64
	CanonicalUncachedTokens              int64
	CanonicalCacheReadTokens             int64
	CanonicalCacheWriteTokens            int64
	CanonicalOutputTokens                int64
	CanonicalNonReasoningTokens          int64
	CanonicalReasoningTokens             int64
	CanonicalUnclassifiedTokens          int64
}

type analyticsIdentityAggregateRow struct {
	AuthType             int
	Identity             string
	Alias                string
	Name                 string
	AuthTypeName         string
	Type                 string
	Provider             string
	Prefix               string
	BaseURL              string
	IsDeleted            bool
	RequestCount         int64
	SuccessCount         int64
	FailureCount         int64
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
	LastUsedAt           string
}

type analyticsIdentityTrendRow struct {
	AuthType             int
	Identity             string
	Bucket               string
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
}

type analyticsModelAggregateRow struct {
	Model                        string
	Provider                     string
	ProviderCount                int64
	RequestCount                 int64
	SuccessCount                 int64
	FailureCount                 int64
	InputTokens                  int64
	OutputTokens                 int64
	ReasoningTokens              int64
	TotalTokens                  int64
	TotalCost                    float64
	CachedTokens                 int64
	CacheReadTokens              int64
	CacheReadObservedInputTokens int64
	CacheSavings                 float64
	CacheSavingsEligibleRows     int64
	CacheSavingsIneligibleRows   int64
	TotalLatencyMS               int64
	LatencySampleCount           int64
	MissingPricingEvents         int64
	PricedBillableEvents         int64
}

type analyticsProviderOptionRow struct {
	Provider             string
	RequestCount         int64
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
}

type analyticsHeatmapAggregateRow struct {
	BucketKey            string
	RequestCount         int64
	FailureCount         int64
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
}

type analyticsIdentityKey struct {
	AuthType int
	Identity string
}

const analyticsKeyAliasBreakdownLimit = 20
