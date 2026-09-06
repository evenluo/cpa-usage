package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func addUsageIdentityPassiveQuotaMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageIdentity{}) {
		return nil
	}
	for _, column := range []string{"passive_quota", "passive_model_quotas"} {
		if tx.Migrator().HasColumn(&entities.UsageIdentity{}, column) {
			continue
		}
		if err := tx.Exec("ALTER TABLE usage_identities ADD COLUMN " + column + " TEXT").Error; err != nil {
			return fmt.Errorf("add usage_identities.%s column: %w", column, err)
		}
	}
	return nil
}
