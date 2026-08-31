package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
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

	// Existing rollups predate the exact cache-read facts. Reset the existing
	// backfill owner so it rebuilds them in bounded batches instead of blocking
	// database startup with a synchronous full-table rewrite.
	if tx.Migrator().HasTable(&entities.UsageRollupBackfillState{}) {
		if err := tx.Exec(`UPDATE usage_rollup_backfill_states
			SET status = ?, target_bucket_start = NULL, covered_bucket_start = NULL,
				started_at = NULL, completed_at = NULL, failed_at = NULL, last_error = ''
			WHERE name = ?`, entities.UsageRollupBackfillStateStatusPending, entities.UsageRollupBackfillStateName).Error; err != nil {
			return fmt.Errorf("reset usage rollup cache-read backfill: %w", err)
		}
	}
	return nil
}
