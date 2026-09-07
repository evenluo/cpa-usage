package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func addUsageRollupAccountingFieldsMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageRollupHourly{}) {
		return nil
	}
	columns := []string{
		"accounting_absent_attempts",
		"accounting_valid_attempts",
		"accounting_valid_complete_attempts",
		"accounting_valid_inconsistent_attempts",
		"accounting_valid_unclassified_attempts",
		"canonical_complete_zero_attempts",
		"canonical_complete_prompt_tokens",
		"canonical_complete_cache_read_tokens",
		"canonical_complete_output_tokens",
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

	// Published main has no canonical facts. Initialize the existing aggregate
	// directly; new inbox events rebuild their affected hours after migration.
	// The migration ledger owns one-time application and checkpoint state stays put.
	if err := tx.Exec("UPDATE usage_rollups_hourly SET accounting_absent_attempts = request_count").Error; err != nil {
		return fmt.Errorf("initialize historical accounting absence: %w", err)
	}
	for _, column := range []string{
		"input_tokens",
		"billable_prompt_tokens",
		"output_tokens",
		"reasoning_tokens",
		"cached_tokens",
		"cache_read_tokens",
		"cache_read_observed_input_tokens",
		"total_tokens",
	} {
		present, err := usageRollupColumnExists(tx, column)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		if err := tx.Exec("ALTER TABLE usage_rollups_hourly DROP COLUMN " + column).Error; err != nil {
			return fmt.Errorf("drop inactive usage_rollups_hourly.%s column: %w", column, err)
		}
	}
	return nil
}

func usageRollupColumnExists(tx *gorm.DB, column string) (bool, error) {
	var count int64
	if err := tx.Raw("SELECT COUNT(*) FROM pragma_table_info('usage_rollups_hourly') WHERE name = ?", column).Scan(&count).Error; err != nil {
		return false, fmt.Errorf("inspect usage_rollups_hourly.%s column: %w", column, err)
	}
	return count != 0, nil
}
