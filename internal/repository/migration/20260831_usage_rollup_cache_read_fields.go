package migration

import (
	"database/sql"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
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

	earliest, latest, hasExactFacts, err := usageEventCacheReadBucketBounds(tx)
	if err != nil {
		return err
	}
	if !hasExactFacts {
		// A same-release upgrade adds nullable event facts before these rollup
		// columns. When all historical facts are NULL, the new zero defaults are
		// already exact and the existing completed owner state must stay intact.
		return nil
	}

	covered := earliest.Add(-time.Hour)
	state := entities.UsageRollupBackfillState{
		Name:               entities.UsageRollupBackfillStateName,
		Status:             entities.UsageRollupBackfillStateStatusPending,
		TargetBucketStart:  &latest,
		CoveredBucketStart: &covered,
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

func usageEventCacheReadBucketBounds(tx *gorm.DB) (time.Time, time.Time, bool, error) {
	var earliest, latest sql.NullString
	row := tx.Raw(`SELECT
		strftime('%Y-%m-%dT%H:00:00Z', MIN(timestamp)),
		strftime('%Y-%m-%dT%H:00:00Z', MAX(timestamp))
		FROM usage_events
		WHERE cache_read_tokens IS NOT NULL`).Row()
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
