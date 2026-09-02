package migration

import (
	"fmt"

	"cpa-usage/internal/entities"

	"gorm.io/gorm"
)

func addUsageIdentityDisabledMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageIdentity{}) {
		return nil
	}
	if tx.Migrator().HasColumn(&entities.UsageIdentity{}, "disabled") {
		return nil
	}
	if err := tx.Exec("ALTER TABLE usage_identities ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT 0").Error; err != nil {
		return fmt.Errorf("add usage_identities.disabled column: %w", err)
	}
	return nil
}
