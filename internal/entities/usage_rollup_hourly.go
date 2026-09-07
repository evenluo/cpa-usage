package entities

import "time"

// UsageRollupHourly stores hourly request aggregates keyed by stable raw-event dimensions.
type UsageRollupHourly struct {
	ID                   uint      `gorm:"primaryKey"`
	BucketStart          time.Time `gorm:"not null;uniqueIndex:uniq_usage_rollups_hourly_dimensions,priority:1;index:idx_usage_rollups_hourly_bucket_provider,priority:1"`
	Provider             string    `gorm:"not null;uniqueIndex:uniq_usage_rollups_hourly_dimensions,priority:2;index:idx_usage_rollups_hourly_bucket_provider,priority:2"`
	Model                string    `gorm:"not null;uniqueIndex:uniq_usage_rollups_hourly_dimensions,priority:3"`
	AuthType             string    `gorm:"not null;uniqueIndex:uniq_usage_rollups_hourly_dimensions,priority:4"`
	AuthIndex            string    `gorm:"not null;uniqueIndex:uniq_usage_rollups_hourly_dimensions,priority:5"`
	APIKeyIdentity       string    `gorm:"not null;uniqueIndex:uniq_usage_rollups_hourly_dimensions,priority:6"`
	RequestCount         int64     `gorm:"not null"`
	SuccessCount         int64     `gorm:"not null"`
	FailureCount         int64     `gorm:"not null"`
	InputTokens          int64     `gorm:"not null"`
	BillablePromptTokens int64     `gorm:"not null"`
	OutputTokens         int64     `gorm:"not null"`
	ReasoningTokens      int64     `gorm:"not null"`
	CachedTokens         int64     `gorm:"not null"`
	CacheReadTokens      int64     `gorm:"not null"`
	// CacheReadObservedInputTokens is the prompt-input denominator from attempts
	// that carried a valid explicit cache_read_tokens fact, including explicit zero.
	CacheReadObservedInputTokens        int64     `gorm:"not null"`
	AccountingAbsentAttempts            int64     `gorm:"not null"`
	AccountingInvalidAttempts           int64     `gorm:"not null"`
	AccountingValidAttempts             int64     `gorm:"not null"`
	AccountingValidCompleteAttempts     int64     `gorm:"not null"`
	AccountingValidInconsistentAttempts int64     `gorm:"not null"`
	AccountingValidUnclassifiedAttempts int64     `gorm:"not null"`
	CanonicalCompleteZeroAttempts       int64     `gorm:"not null"`
	CanonicalCompletePromptTokens       int64     `gorm:"not null"`
	CanonicalCompleteCacheReadTokens    int64     `gorm:"not null"`
	CanonicalCompleteOutputTokens       int64     `gorm:"not null"`
	CanonicalTotalTokens                int64     `gorm:"not null"`
	CanonicalInputTokens                int64     `gorm:"not null"`
	CanonicalUncachedTokens             int64     `gorm:"not null"`
	CanonicalCacheReadTokens            int64     `gorm:"not null"`
	CanonicalCacheWriteTokens           int64     `gorm:"not null"`
	CanonicalOutputTokens               int64     `gorm:"not null"`
	CanonicalNonReasoningTokens         int64     `gorm:"not null"`
	CanonicalReasoningTokens            int64     `gorm:"not null"`
	CanonicalUnclassifiedTokens         int64     `gorm:"not null"`
	TotalTokens                         int64     `gorm:"not null"`
	TotalLatencyMS                      int64     `gorm:"not null"`
	LatencySampleCount                  int64     `gorm:"not null"`
	LastEventAt                         time.Time `gorm:"not null"`
	CreatedAt                           time.Time
	UpdatedAt                           time.Time
}

func (UsageRollupHourly) TableName() string {
	return "usage_rollups_hourly"
}
