package migration

import (
	"fmt"

	"cpa-usage/internal/entities"

	"gorm.io/gorm"
)

func addUsageIdentityAvailabilityMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageIdentity{}) {
		return nil
	}
	columns := []struct {
		name       string
		definition string
	}{
		{name: "auth_file_status", definition: "TEXT"},
		{name: "unavailable", definition: "BOOLEAN"},
		{name: "last_refresh", definition: "DATETIME"},
		{name: "next_retry_after", definition: "DATETIME"},
		{name: "metadata_observed_at", definition: "DATETIME"},
	}
	for _, column := range columns {
		if tx.Migrator().HasColumn(&entities.UsageIdentity{}, column.name) {
			continue
		}
		if err := tx.Exec("ALTER TABLE usage_identities ADD COLUMN " + column.name + " " + column.definition).Error; err != nil {
			return fmt.Errorf("add usage_identities.%s column: %w", column.name, err)
		}
	}
	return nil
}
