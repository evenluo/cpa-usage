package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func addUsageAttemptFieldsMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageEvent{}) {
		return nil
	}
	columns := []struct {
		name       string
		definition string
	}{
		{name: "status_code", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "executor_type", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "reasoning_effort", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "service_tier", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "cache_read_tokens", definition: "INTEGER"},
		{name: "cache_creation_tokens", definition: "INTEGER"},
	}
	for _, column := range columns {
		if tx.Migrator().HasColumn(&entities.UsageEvent{}, column.name) {
			continue
		}
		if err := tx.Exec("ALTER TABLE usage_events ADD COLUMN " + column.name + " " + column.definition).Error; err != nil {
			return fmt.Errorf("add usage_events.%s column: %w", column.name, err)
		}
	}
	return nil
}
