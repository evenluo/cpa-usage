package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func addUsageRollupCacheReadFieldsMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageRollupHourly{}) {
		return nil
	}
	columns := []struct {
		name       string
		definition string
	}{
		{name: "cache_read_tokens", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "cache_read_observed_input_tokens", definition: "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		if tx.Migrator().HasColumn(&entities.UsageRollupHourly{}, column.name) {
			continue
		}
		if err := tx.Exec("ALTER TABLE usage_rollups_hourly ADD COLUMN " + column.name + " " + column.definition).Error; err != nil {
			return fmt.Errorf("add usage_rollups_hourly.%s column: %w", column.name, err)
		}
	}

	if !tx.Migrator().HasTable(&entities.UsageRollupBackfillState{}) ||
		!tx.Migrator().HasTable(&entities.UsageEvent{}) ||
		!tx.Migrator().HasColumn(&entities.UsageEvent{}, "cache_read_tokens") {
		return nil
	}

	earliest, latest, hasValidFacts, err := validUsageEventCacheReadBucketBounds(tx)
	if err != nil {
		return err
	}
	if !hasValidFacts {
		// A same-release upgrade adds nullable event facts before these rollup
		// columns. When no historical facts are valid analytics observations, the
		// new zero defaults are already exact and owner state must stay intact.
		return nil
	}

	var existing entities.UsageRollupBackfillState
	existingErr := tx.Where("name = ?", entities.UsageRollupBackfillStateName).First(&existing).Error
	if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return fmt.Errorf("load usage rollup backfill state before cache-read migration: %w", existingErr)
	}

	state := entities.UsageRollupBackfillState{
		Name:   entities.UsageRollupBackfillStateName,
		Status: entities.UsageRollupBackfillStateStatusPending,
	}
	exactCovered := earliest.Add(-time.Hour)
	if existingErr == nil && completedRollupBackfillStateCoversTarget(existing) {
		state.TargetBucketStart = &latest
		state.CoveredBucketStart = &exactCovered
	} else {
		if existing.CoveredBucketStart != nil {
			covered := existing.CoveredBucketStart.UTC().Truncate(time.Hour)
			if exactCovered.Before(covered) {
				covered = exactCovered
			}
			state.CoveredBucketStart = &covered
		}
		if existing.TargetBucketStart != nil {
			target := existing.TargetBucketStart.UTC().Truncate(time.Hour)
			if latest.After(target) {
				target = latest
			}
			state.TargetBucketStart = &target
		} else {
			latestEvent, err := latestUsageEventBucket(tx)
			if err != nil {
				return err
			}
			state.TargetBucketStart = &latestEvent
		}
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
		return fmt.Errorf("schedule usage rollup cache-read backfill: %w", err)
	}
	return nil
}

func completedRollupBackfillStateCoversTarget(state entities.UsageRollupBackfillState) bool {
	return state.Status == repodto.RollupBackfillStatusCompleted &&
		state.TargetBucketStart != nil &&
		state.CoveredBucketStart != nil &&
		!state.CoveredBucketStart.UTC().Truncate(time.Hour).Before(state.TargetBucketStart.UTC().Truncate(time.Hour))
}

func validUsageEventCacheReadBucketBounds(tx *gorm.DB) (time.Time, time.Time, bool, error) {
	var earliest, latest sql.NullString
	row := tx.Raw(`SELECT
		strftime('%Y-%m-%dT%H:00:00Z', MIN(timestamp)),
		strftime('%Y-%m-%dT%H:00:00Z', MAX(timestamp))
		FROM usage_events
		WHERE input_tokens > 0
			AND cache_read_tokens IS NOT NULL
			AND cache_read_tokens >= 0
			AND cache_read_tokens <= input_tokens`).Row()
	if err := row.Scan(&earliest, &latest); err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("load usage event cache-read bucket bounds: %w", err)
	}
	if !earliest.Valid || !latest.Valid {
		return time.Time{}, time.Time{}, false, nil
	}
	earliestBucket, err := time.Parse(time.RFC3339, earliest.String)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parse earliest usage event cache-read bucket %q: %w", earliest.String, err)
	}
	latestBucket, err := time.Parse(time.RFC3339, latest.String)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parse latest usage event cache-read bucket %q: %w", latest.String, err)
	}
	return earliestBucket, latestBucket, true, nil
}

func latestUsageEventBucket(tx *gorm.DB) (time.Time, error) {
	var latest sql.NullString
	row := tx.Raw(`SELECT strftime('%Y-%m-%dT%H:00:00Z', MAX(timestamp)) FROM usage_events`).Row()
	if err := row.Scan(&latest); err != nil {
		return time.Time{}, fmt.Errorf("load latest usage event bucket: %w", err)
	}
	if !latest.Valid {
		return time.Time{}, fmt.Errorf("load latest usage event bucket: no usage events")
	}
	bucket, err := time.Parse(time.RFC3339, latest.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse latest usage event bucket %q: %w", latest.String, err)
	}
	return bucket, nil
}
