package migration

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

const (
	migrationAddUsageEventRedisFields               = "20260503_add_usage_event_redis_fields"
	migrationBackfillUsageEventRedisFields          = "20260503_backfill_usage_event_redis_fields"
	migrationDropSnapshotRuns                       = "20260503_drop_snapshot_runs"
	migrationDropLegacySnapshotRunColumns           = "20260504_drop_legacy_snapshot_run_columns"
	migrationCreateUsageIdentities                  = "20260504_create_usage_identities"
	migrationMigrateUsageIdentitiesMetadata         = "20260504_migrate_usage_identities_metadata"
	migrationBackfillUsageEventIdentityFields       = "20260504_backfill_usage_event_identity_fields"
	migrationBackfillUsageIdentityStats             = "20260504_backfill_usage_identity_stats"
	migrationDropLegacyMetadataTables               = "20260504_drop_legacy_metadata_tables"
	migrationRemovePrefixUsageIdentities            = "20260504_remove_prefix_usage_identities"
	migrationAddUsageIdentityLookupKey              = "20260505_add_usage_identity_lookup_key"
	migrationMigrateAIProviderIdentitiesToAuthIndex = "20260505_migrate_ai_provider_identities_to_auth_index"
	migrationAddUsagePerformanceIndexes             = "20260506_add_usage_performance_indexes"
	migrationAddUsageIdentityMetadataFields         = "20260507_add_usage_identity_metadata_fields"
	migrationAddUsageEventModelAlias                = "20260508_add_usage_event_model_alias"
	migrationUpdateUsageIdentityQuotaFields         = "20260509_update_usage_identity_quota_fields"
	migrationRemoveUsageIdentityQuotaFields         = "20260510_remove_usage_identity_quota_fields"
	migrationAddUsageIdentityBaseURL                = "20260511_add_usage_identity_base_url"
	migrationCreateKeyAliases                       = "20260513_create_key_aliases"
	migrationEnsureUsageEventEventKeyUnique         = "20260518_ensure_usage_event_event_key_unique"
	migrationCreateUsageRollupBackfillState         = "20260707_create_usage_rollup_backfill_state"
	migrationCreateUsageRollupsHourly               = "20260707_create_usage_rollups_hourly"
	migrationAddRedisInboxProcessableIndex          = "20260707_add_redis_inbox_processable_index"
	migrationAddUsageEventTTFT                      = "20260714_add_usage_event_ttft"
	migrationAddUsageAttemptFields                  = "20260831_add_usage_attempt_fields"
	migrationAddUsageRollupCacheReadFields          = "20260831_add_usage_rollup_cache_read_fields"
	migrationAddUsageIdentityDisabled               = "20260902_add_usage_identity_disabled"
)

type schemaMigration struct {
	Version   string    `gorm:"primaryKey;column:version"`
	AppliedAt time.Time `gorm:"not null;column:applied_at"`
}

func (schemaMigration) TableName() string {
	return "schema_migrations"
}

type databaseMigration struct {
	version string
	run     func(*gorm.DB) error
}

func Run(db *gorm.DB) error {
	if err := createSchemaMigrationsTable(db); err != nil {
		return err
	}

	for _, migration := range orderedMigrations() {
		if err := runSchemaMigration(db, migration); err != nil {
			return err
		}
	}
	return nil
}

func MarkAllAsApplied(db *gorm.DB) error {
	if err := createSchemaMigrationsTable(db); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		for _, migration := range orderedMigrations() {
			if err := tx.Exec("INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)", migration.version, now).Error; err != nil {
				return fmt.Errorf("mark schema migration %s applied: %w", migration.version, err)
			}
		}
		return nil
	})
}

func createSchemaMigrationsTable(db *gorm.DB) error {
	if err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME NOT NULL)").Error; err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}
	return nil
}

func orderedMigrations() []databaseMigration {
	return []databaseMigration{
		{version: migrationAddUsageEventRedisFields, run: addUsageEventRedisFieldsMigration},
		{version: migrationBackfillUsageEventRedisFields, run: backfillUsageEventRedisFieldsMigration},
		{version: migrationDropSnapshotRuns, run: dropSnapshotRunsMigration},
		{version: migrationDropLegacySnapshotRunColumns, run: dropLegacySnapshotRunColumnsMigration},
		{version: migrationCreateUsageIdentities, run: createUsageIdentitiesMigration},
		{version: migrationMigrateUsageIdentitiesMetadata, run: migrateUsageIdentitiesMetadataMigration},
		{version: migrationBackfillUsageEventIdentityFields, run: backfillUsageEventIdentityFieldsMigration},
		{version: migrationBackfillUsageIdentityStats, run: backfillUsageIdentityStatsMigration},
		{version: migrationDropLegacyMetadataTables, run: dropLegacyMetadataTablesMigration},
		{version: migrationRemovePrefixUsageIdentities, run: removePrefixUsageIdentitiesMigration},
		{version: migrationAddUsageIdentityLookupKey, run: addUsageIdentityLookupKeyMigration},
		{version: migrationMigrateAIProviderIdentitiesToAuthIndex, run: migrateAIProviderIdentitiesToAuthIndexMigration},
		{version: migrationAddUsagePerformanceIndexes, run: addUsagePerformanceIndexesMigration},
		{version: migrationAddUsageIdentityMetadataFields, run: addUsageIdentityMetadataFieldsMigration},
		{version: migrationAddUsageEventModelAlias, run: addUsageEventModelAliasMigration},
		{version: migrationUpdateUsageIdentityQuotaFields, run: updateUsageIdentityQuotaFieldsMigration},
		{version: migrationRemoveUsageIdentityQuotaFields, run: removeUsageIdentityQuotaFieldsMigration},
		{version: migrationAddUsageIdentityBaseURL, run: addUsageIdentityBaseURLMigration},
		{version: migrationCreateKeyAliases, run: createKeyAliasesMigration},
		{version: migrationEnsureUsageEventEventKeyUnique, run: ensureUsageEventEventKeyUniqueMigration},
		{version: migrationCreateUsageRollupBackfillState, run: createUsageRollupBackfillStateMigration},
		{version: migrationCreateUsageRollupsHourly, run: createUsageRollupsHourlyMigration},
		{version: migrationAddRedisInboxProcessableIndex, run: addRedisInboxProcessableIndexMigration},
		{version: migrationAddUsageEventTTFT, run: addUsageEventTTFTMigration},
		{version: migrationAddUsageAttemptFields, run: addUsageAttemptFieldsMigration},
		{version: migrationAddUsageRollupCacheReadFields, run: addUsageRollupCacheReadFieldsMigration},
		{version: migrationAddUsageIdentityDisabled, run: addUsageIdentityDisabledMigration},
	}
}

func runSchemaMigration(db *gorm.DB, migration databaseMigration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("schema_migrations").Where("version = ?", migration.version).Count(&count).Error; err != nil {
			slog.Error("schema migration failed", "version", migration.version, "error", err)
			return fmt.Errorf("check schema migration %s: %w", migration.version, err)
		}
		if count > 0 {
			slog.Info("schema migration skipped", "version", migration.version)
			return nil
		}
		slog.Info("schema migration started", "version", migration.version)
		if err := migration.run(tx); err != nil {
			slog.Error("schema migration failed", "version", migration.version, "error", err)
			return fmt.Errorf("run schema migration %s: %w", migration.version, err)
		}
		if err := tx.Create(&schemaMigration{Version: migration.version, AppliedAt: time.Now().UTC()}).Error; err != nil {
			slog.Error("schema migration failed", "version", migration.version, "error", err)
			return fmt.Errorf("record schema migration %s: %w", migration.version, err)
		}
		slog.Info("schema migration applied", "version", migration.version)
		return nil
	})
}
