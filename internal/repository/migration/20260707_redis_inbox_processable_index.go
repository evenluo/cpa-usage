package migration

import "gorm.io/gorm"

func addRedisInboxProcessableIndexMigration(tx *gorm.DB) error {
	if !hasMigrationColumns(tx, "redis_usage_inboxes", "id", "status") {
		return nil
	}
	return tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_redis_usage_inboxes_processable_id
		ON redis_usage_inboxes(id)
		WHERE status = 'pending' OR status = 'process_failed'
	`).Error
}
