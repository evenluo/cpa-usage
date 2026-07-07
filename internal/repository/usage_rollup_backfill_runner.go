package repository

import (
	"errors"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

const DefaultUsageRollupBackfillBatchHours = 24

type UsageRollupBackfillBatchResult struct {
	Status             repodto.RollupBackfillStatus
	Done               bool
	BatchStart         *time.Time
	BatchEnd           *time.Time
	RebuiltBucketCount int
}

func BackfillUsageRollupsBatch(db *gorm.DB, now time.Time, batchHours int) (UsageRollupBackfillBatchResult, error) {
	if db == nil {
		return UsageRollupBackfillBatchResult{}, fmt.Errorf("database is nil")
	}
	if batchHours <= 0 {
		batchHours = DefaultUsageRollupBackfillBatchHours
	}

	status, err := GetUsageRollupBackfillStatus(db)
	if err != nil {
		return UsageRollupBackfillBatchResult{}, err
	}
	if status.Status == repodto.RollupBackfillStatusCompleted && rollupBackfillTargetCovered(status) {
		return UsageRollupBackfillBatchResult{Status: status, Done: true}, nil
	}

	clock := now.UTC()
	if clock.IsZero() {
		clock = time.Now().UTC()
	}
	target := rollupBackfillTarget(status, clock)
	earliest, hasEvents, err := earliestUsageEventBucket(db)
	if err != nil {
		return UsageRollupBackfillBatchResult{}, err
	}
	startedAt := status.StartedAt
	if startedAt == nil {
		started := clock
		startedAt = &started
	}
	covered := status.CoveredBucketStart

	if !hasEvents || target.Before(earliest) {
		completed := clock
		status = repodto.RollupBackfillStatus{
			Status:             repodto.RollupBackfillStatusCompleted,
			TargetBucketStart:  &target,
			CoveredBucketStart: &target,
			StartedAt:          startedAt,
			CompletedAt:        &completed,
		}
		if err := SaveUsageRollupBackfillStatus(db, status); err != nil {
			return UsageRollupBackfillBatchResult{}, err
		}
		return UsageRollupBackfillBatchResult{Status: status, Done: true}, nil
	}

	next := earliest
	if covered != nil {
		next = covered.UTC().Truncate(time.Hour).Add(time.Hour)
	}
	if next.After(target) {
		completed := clock
		status = repodto.RollupBackfillStatus{
			Status:             repodto.RollupBackfillStatusCompleted,
			TargetBucketStart:  &target,
			CoveredBucketStart: &target,
			StartedAt:          startedAt,
			CompletedAt:        &completed,
		}
		if err := SaveUsageRollupBackfillStatus(db, status); err != nil {
			return UsageRollupBackfillBatchResult{}, err
		}
		return UsageRollupBackfillBatchResult{Status: status, Done: true}, nil
	}

	batchEnd := next.Add(time.Duration(batchHours-1) * time.Hour)
	if batchEnd.After(target) {
		batchEnd = target
	}
	running := repodto.RollupBackfillStatus{
		Status:             repodto.RollupBackfillStatusRunning,
		TargetBucketStart:  &target,
		CoveredBucketStart: covered,
		StartedAt:          startedAt,
	}
	if err := SaveUsageRollupBackfillStatus(db, running); err != nil {
		return UsageRollupBackfillBatchResult{}, err
	}

	var result UsageRollupBackfillBatchResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := SaveUsageRollupBackfillStatus(tx, running); err != nil {
			return err
		}
		if err := rebuildUsageRollupsForBucketRange(tx, next, batchEnd); err != nil {
			return err
		}

		nextCovered := batchEnd
		done := !nextCovered.Before(target)
		status = repodto.RollupBackfillStatus{
			Status:             repodto.RollupBackfillStatusRunning,
			TargetBucketStart:  &target,
			CoveredBucketStart: &nextCovered,
			StartedAt:          startedAt,
		}
		if done {
			completed := clock
			status.Status = repodto.RollupBackfillStatusCompleted
			status.CompletedAt = &completed
		}
		if err := SaveUsageRollupBackfillStatus(tx, status); err != nil {
			return err
		}
		result = UsageRollupBackfillBatchResult{
			Status:             status,
			Done:               done,
			BatchStart:         &next,
			BatchEnd:           &batchEnd,
			RebuiltBucketCount: len(hourlyBucketRange(next, batchEnd)),
		}
		return nil
	}); err != nil {
		failedAt := clock
		failed := running
		failed.Status = repodto.RollupBackfillStatusFailed
		failed.FailedAt = &failedAt
		failed.LastError = err.Error()
		_ = SaveUsageRollupBackfillStatus(db, failed)
		return UsageRollupBackfillBatchResult{Status: failed, BatchStart: &next, BatchEnd: &batchEnd}, err
	}
	return result, nil
}

func rollupBackfillTarget(status repodto.RollupBackfillStatus, now time.Time) time.Time {
	if status.TargetBucketStart != nil {
		return status.TargetBucketStart.UTC().Truncate(time.Hour)
	}
	return now.UTC().Truncate(time.Hour).Add(-time.Hour)
}

func rollupBackfillTargetCovered(status repodto.RollupBackfillStatus) bool {
	if status.TargetBucketStart == nil || status.CoveredBucketStart == nil {
		return false
	}
	return !status.CoveredBucketStart.UTC().Truncate(time.Hour).Before(status.TargetBucketStart.UTC().Truncate(time.Hour))
}

func earliestUsageEventBucket(db *gorm.DB) (time.Time, bool, error) {
	var event entities.UsageEvent
	err := db.Order("timestamp ASC").Limit(1).First(&event).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("load earliest usage event for rollup backfill: %w", err)
	}
	return event.Timestamp.UTC().Truncate(time.Hour), true, nil
}
