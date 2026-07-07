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
	TotalTokens          int64     `gorm:"not null"`
	TotalLatencyMS       int64     `gorm:"not null"`
	LatencySampleCount   int64     `gorm:"not null"`
	LastEventAt          time.Time `gorm:"not null"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (UsageRollupHourly) TableName() string {
	return "usage_rollups_hourly"
}
