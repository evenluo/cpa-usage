package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func addUsageAccountingFieldsMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageEvent{}) {
		return nil
	}
	// The published consumer must finish its durable inbox before the schema
	// cut. These rows contain its reduced projection, not the required v2 facts.
	if tx.Migrator().HasTable(&entities.RedisUsageInbox{}) {
		var pending int64
		if err := tx.Model(&entities.RedisUsageInbox{}).Where("status IN ?", []string{"pending", "process_failed"}).Count(&pending).Error; err != nil {
			return fmt.Errorf("check inbox before accounting v2 cutover: %w", err)
		}
		if pending != 0 {
			return fmt.Errorf("accounting v2 cutover requires an empty processable inbox (%d rows): finish inbox processing with the deployed consumer before upgrading", pending)
		}
	}
	columns := []struct{ name, definition string }{
		{"accounting_version", "INTEGER"},
		{"accounting_state", "TEXT NOT NULL DEFAULT 'absent'"},
		{"token_schema_version", "INTEGER"},
		{"token_quality", "TEXT"},
		{"canonical_total_tokens", "INTEGER"},
		{"canonical_input_tokens", "INTEGER"},
		{"canonical_uncached_tokens", "INTEGER"},
		{"canonical_cache_read_tokens", "INTEGER"},
		{"canonical_cache_write_tokens", "INTEGER"},
		{"canonical_output_tokens", "INTEGER"},
		{"canonical_non_reasoning_tokens", "INTEGER"},
		{"canonical_reasoning_tokens", "INTEGER"},
		{"canonical_unclassified_tokens", "INTEGER"},
		{"generate", "numeric"},
		{"stream", "numeric"},
		{"response_service_tier", "TEXT"},
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
