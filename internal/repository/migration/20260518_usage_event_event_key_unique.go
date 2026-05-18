package migration

import (
	"fmt"

	"gorm.io/gorm"
)

func ensureUsageEventEventKeyUniqueMigration(db *gorm.DB) error {
	if !hasMigrationColumns(db, "usage_events", "event_key") {
		return fmt.Errorf("missing required schema for usage event event_key unique migration")
	}

	var duplicateCount int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT event_key
			FROM usage_events
			GROUP BY event_key
			HAVING COUNT(*) > 1
		)
	`).Scan(&duplicateCount).Error; err != nil {
		return fmt.Errorf("check duplicate usage event keys: %w", err)
	}
	if duplicateCount > 0 {
		return fmt.Errorf("usage_events.event_key contains duplicate values")
	}

	statements := []string{
		"DROP INDEX IF EXISTS idx_usage_events_event_key",
		"DROP INDEX IF EXISTS uniq_usage_events_event_key",
		"CREATE UNIQUE INDEX uniq_usage_events_event_key ON usage_events(event_key)",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
