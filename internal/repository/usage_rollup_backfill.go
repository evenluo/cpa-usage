package repository

import (
	"errors"
	"fmt"

	"cpa-usage/internal/entities"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func EnsureUsageRollupBackfillState(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	state := entities.UsageRollupBackfillState{
		Name:   entities.UsageRollupBackfillStateName,
		Status: entities.UsageRollupBackfillStateStatusPending,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
		return fmt.Errorf("ensure usage rollup backfill state: %w", err)
	}
	return nil
}

func GetUsageRollupBackfillStatus(db *gorm.DB) (repodto.RollupBackfillStatus, error) {
	if db == nil {
		return repodto.RollupBackfillStatus{}, fmt.Errorf("database is nil")
	}

	var state entities.UsageRollupBackfillState
	err := db.Where("name = ?", entities.UsageRollupBackfillStateName).First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repodto.PendingRollupBackfillStatus(), nil
		}
		return repodto.RollupBackfillStatus{}, fmt.Errorf("load usage rollup backfill state: %w", err)
	}
	return rollupBackfillStatusFromEntity(state), nil
}

func SaveUsageRollupBackfillStatus(db *gorm.DB, status repodto.RollupBackfillStatus) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	status = repodto.NormalizeRollupBackfillStatus(status)

	state := entities.UsageRollupBackfillState{
		Name:               entities.UsageRollupBackfillStateName,
		Status:             status.Status,
		TargetBucketStart:  status.TargetBucketStart,
		CoveredBucketStart: status.CoveredBucketStart,
		StartedAt:          status.StartedAt,
		CompletedAt:        status.CompletedAt,
		FailedAt:           status.FailedAt,
		LastError:          status.LastError,
	}
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status",
			"target_bucket_start",
			"covered_bucket_start",
			"started_at",
			"completed_at",
			"failed_at",
			"last_error",
			"updated_at",
		}),
	}).Create(&state).Error
	if err != nil {
		return fmt.Errorf("save usage rollup backfill state: %w", err)
	}
	return nil
}

func rollupBackfillStatusFromEntity(state entities.UsageRollupBackfillState) repodto.RollupBackfillStatus {
	return repodto.NormalizeRollupBackfillStatus(repodto.RollupBackfillStatus{
		Status:             state.Status,
		TargetBucketStart:  state.TargetBucketStart,
		CoveredBucketStart: state.CoveredBucketStart,
		StartedAt:          state.StartedAt,
		CompletedAt:        state.CompletedAt,
		FailedAt:           state.FailedAt,
		LastError:          state.LastError,
	})
}
