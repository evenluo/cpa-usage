package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func addUsageIdentityAuthRenewalMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageIdentity{}) {
		return nil
	}
	if tx.Migrator().HasColumn(&entities.UsageIdentity{}, "auth_renewal") {
		return nil
	}
	if err := tx.Exec("ALTER TABLE usage_identities ADD COLUMN auth_renewal TEXT").Error; err != nil {
		return fmt.Errorf("add usage_identities.auth_renewal column: %w", err)
	}
	return nil
}
