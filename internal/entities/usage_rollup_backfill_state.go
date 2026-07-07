package entities

import "time"

const (
	UsageRollupBackfillStateName          = "usage_rollup_hourly"
	UsageRollupBackfillStateStatusPending = "pending"
)

// UsageRollupBackfillState stores the singleton progress row for hourly Usage Rollup backfill.
type UsageRollupBackfillState struct {
	Name               string     `gorm:"primaryKey;size:64"`
	Status             string     `gorm:"not null;size:32;index"`
	TargetBucketStart  *time.Time `gorm:"index"`
	CoveredBucketStart *time.Time `gorm:"index"`
	StartedAt          *time.Time
	CompletedAt        *time.Time
	FailedAt           *time.Time
	LastError          string `gorm:"type:text"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (UsageRollupBackfillState) TableName() string {
	return "usage_rollup_backfill_states"
}
