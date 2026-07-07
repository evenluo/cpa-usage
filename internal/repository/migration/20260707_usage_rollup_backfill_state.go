package migration

import (
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func createUsageRollupBackfillStateMigration(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&entities.UsageRollupBackfillState{}); err != nil {
		return fmt.Errorf("create usage_rollup_backfill_states table: %w", err)
	}
	if err := seedUsageRollupBackfillState(tx); err != nil {
		return err
	}
	return nil
}

func seedUsageRollupBackfillState(tx *gorm.DB) error {
	now := time.Now().UTC()
	state := entities.UsageRollupBackfillState{
		Name:      entities.UsageRollupBackfillStateName,
		Status:    entities.UsageRollupBackfillStateStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.FirstOrCreate(&state, entities.UsageRollupBackfillState{Name: entities.UsageRollupBackfillStateName}).Error; err != nil {
		return fmt.Errorf("seed usage_rollup_backfill_states pending row: %w", err)
	}
	return nil
}
