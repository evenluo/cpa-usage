package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func addUsageRollupAccountingFieldsMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageRollupHourly{}) {
		return nil
	}
	columns := []string{
		"accounting_absent_attempts",
		"accounting_malformed_attempts",
		"accounting_unsupported_version_attempts",
		"accounting_unsupported_schema_attempts",
		"accounting_missing_attempts",
		"accounting_unknown_quality_attempts",
		"accounting_invalid_attempts",
		"accounting_valid_attempts",
		"accounting_valid_complete_attempts",
		"accounting_valid_inconsistent_attempts",
		"accounting_valid_unclassified_attempts",
		"canonical_total_tokens",
		"canonical_input_tokens",
		"canonical_uncached_tokens",
		"canonical_cache_read_tokens",
		"canonical_cache_write_tokens",
		"canonical_output_tokens",
		"canonical_non_reasoning_tokens",
		"canonical_reasoning_tokens",
		"canonical_unclassified_tokens",
	}
	for _, column := range columns {
		if tx.Migrator().HasColumn(&entities.UsageRollupHourly{}, column) {
			continue
		}
		if err := tx.Exec("ALTER TABLE usage_rollups_hourly ADD COLUMN " + column + " INTEGER NOT NULL DEFAULT 0").Error; err != nil {
			return fmt.Errorf("add usage_rollups_hourly.%s column: %w", column, err)
		}
	}

	if !tx.Migrator().HasTable(&entities.UsageRollupBackfillState{}) ||
		!tx.Migrator().HasTable(&entities.UsageEvent{}) {
		return nil
	}
	earliest, latest, hasEvents, err := usageEventBucketBounds(tx)
	if err != nil || !hasEvents {
		return err
	}

	var existing entities.UsageRollupBackfillState
	existingErr := tx.Where("name = ?", entities.UsageRollupBackfillStateName).First(&existing).Error
	if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return fmt.Errorf("load usage rollup backfill state before accounting migration: %w", existingErr)
	}
	target := latest
	if existing.TargetBucketStart != nil && existing.TargetBucketStart.UTC().Truncate(time.Hour).After(target) {
		target = existing.TargetBucketStart.UTC().Truncate(time.Hour)
	}
	state := entities.UsageRollupBackfillState{
		Name:              entities.UsageRollupBackfillStateName,
		Status:            entities.UsageRollupBackfillStateStatusPending,
		TargetBucketStart: &target,
	}
	if existing.CoveredBucketStart != nil {
		covered := existing.CoveredBucketStart.UTC().Truncate(time.Hour)
		exactCovered := earliest.Add(-time.Hour)
		if exactCovered.Before(covered) {
			covered = exactCovered
		}
		state.CoveredBucketStart = &covered
	}
	if err := tx.Clauses(clause.OnConflict{
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
	}).Create(&state).Error; err != nil {
		return fmt.Errorf("schedule usage rollup accounting backfill: %w", err)
	}
	return nil
}

func usageEventBucketBounds(tx *gorm.DB) (time.Time, time.Time, bool, error) {
	var earliest, latest sql.NullString
	row := tx.Raw(`SELECT
		strftime('%Y-%m-%dT%H:00:00Z', (SELECT MIN(timestamp) FROM usage_events)),
		strftime('%Y-%m-%dT%H:00:00Z', (SELECT MAX(timestamp) FROM usage_events))`).Row()
	if err := row.Scan(&earliest, &latest); err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("load usage event bucket bounds: %w", err)
	}
	if !earliest.Valid || !latest.Valid {
		return time.Time{}, time.Time{}, false, nil
	}
	earliestBucket, err := time.Parse(time.RFC3339, earliest.String)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parse earliest usage event bucket %q: %w", earliest.String, err)
	}
	latestBucket, err := time.Parse(time.RFC3339, latest.String)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parse latest usage event bucket %q: %w", latest.String, err)
	}
	return earliestBucket, latestBucket, true, nil
}
