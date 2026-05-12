package migration

import (
	"fmt"

	"cpa-usage/internal/entities"

	"gorm.io/gorm"
)

func createKeyAliasesMigration(tx *gorm.DB) error {
	if tx.Migrator().HasTable(&entities.KeyAlias{}) {
		return nil
	}
	if err := tx.AutoMigrate(&entities.KeyAlias{}); err != nil {
		return fmt.Errorf("create key_aliases table: %w", err)
	}
	return nil
}
