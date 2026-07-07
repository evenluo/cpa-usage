package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func createUsageRollupsHourlyMigration(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&entities.UsageRollupHourly{}); err != nil {
		return fmt.Errorf("create usage_rollups_hourly table: %w", err)
	}
	return nil
}
