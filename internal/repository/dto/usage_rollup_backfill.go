package dto

import "time"

const (
	RollupBackfillStatusPending   = "pending"
	RollupBackfillStatusRunning   = "running"
	RollupBackfillStatusCompleted = "completed"
	RollupBackfillStatusFailed    = "failed"
)

type RollupBackfillStatus struct {
	Status             string
	TargetBucketStart  *time.Time
	CoveredBucketStart *time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
	FailedAt           *time.Time
	LastError          string
}

func PendingRollupBackfillStatus() RollupBackfillStatus {
	return RollupBackfillStatus{Status: RollupBackfillStatusPending}
}

func NormalizeRollupBackfillStatus(status RollupBackfillStatus) RollupBackfillStatus {
	if status.Status == "" {
		status.Status = RollupBackfillStatusPending
	}
	return status
}
