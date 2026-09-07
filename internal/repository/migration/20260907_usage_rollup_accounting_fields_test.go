package migration

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAccountingUpgradeInitializesHistoryWithoutResettingCoverage(t *testing.T) {
	for _, status := range []string{"completed", "pending", "failed"} {
		t.Run(status, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "accounting-rollup.db"))), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenedDatabase(t, db)
			if err := db.Exec(`CREATE TABLE usage_rollups_hourly (
				id integer PRIMARY KEY AUTOINCREMENT,
				bucket_start datetime NOT NULL,
				provider text NOT NULL,
				model text NOT NULL,
				auth_type text NOT NULL,
				auth_index text NOT NULL,
				api_key_identity text NOT NULL,
				request_count integer NOT NULL,
				success_count integer NOT NULL,
				failure_count integer NOT NULL,
				input_tokens integer NOT NULL,
				billable_prompt_tokens integer NOT NULL,
				output_tokens integer NOT NULL,
				reasoning_tokens integer NOT NULL,
				cached_tokens integer NOT NULL,
				cache_read_tokens integer NOT NULL,
				cache_read_observed_input_tokens integer NOT NULL,
				total_tokens integer NOT NULL,
				total_latency_ms integer NOT NULL,
				latency_sample_count integer NOT NULL,
				last_event_at datetime NOT NULL,
				created_at datetime,
				updated_at datetime
			)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`CREATE UNIQUE INDEX uniq_usage_rollups_hourly_dimensions ON usage_rollups_hourly(bucket_start, provider, model, auth_type, auth_index, api_key_identity)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`CREATE INDEX idx_usage_rollups_hourly_bucket_provider ON usage_rollups_hourly(bucket_start, provider)`).Error; err != nil {
				t.Fatal(err)
			}
			bucket1 := time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)
			bucket2 := bucket1.Add(time.Hour)
			if err := db.Exec(`INSERT INTO usage_rollups_hourly (
				id, bucket_start, provider, model, auth_type, auth_index, api_key_identity,
				request_count, success_count, failure_count, input_tokens, billable_prompt_tokens,
				output_tokens, reasoning_tokens, cached_tokens, cache_read_tokens,
				cache_read_observed_input_tokens, total_tokens, total_latency_ms,
				latency_sample_count, last_event_at
			) VALUES
				(1, ?, 'p', 'm', 'oauth', 'a', '', 7, 6, 1, 800, 700, 300, 50, 20, 20, 800, 1200, 70, 1, ?),
				(2, ?, 'p', 'm2', 'oauth', 'a', '', 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, ?)`, bucket1, bucket1, bucket2, bucket2).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&entities.UsageRollupBackfillState{}); err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
			state := entities.UsageRollupBackfillState{
				Name: entities.UsageRollupBackfillStateName, Status: status,
				TargetBucketStart: &at, CoveredBucketStart: &at,
				StartedAt: &at, CompletedAt: &at, FailedAt: &at, LastError: "existing observation",
			}
			if err := db.Create(&state).Error; err != nil {
				t.Fatal(err)
			}
			var before entities.UsageRollupBackfillState
			if err := db.First(&before).Error; err != nil {
				t.Fatal(err)
			}
			if err := createSchemaMigrationsTable(db); err != nil {
				t.Fatal(err)
			}
			migration := databaseMigration{version: migrationAddUsageRollupAccountingFields, run: addUsageRollupAccountingFieldsMigration}
			if err := runSchemaMigration(db, migration); err != nil {
				t.Fatal(err)
			}
			var after entities.UsageRollupBackfillState
			if err := db.First(&after).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("coverage changed: before=%+v after=%+v", before, after)
			}
			var rows []entities.UsageRollupHourly
			if err := db.Order("id").Find(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 || rows[0].AccountingAbsentAttempts != 7 || rows[1].AccountingAbsentAttempts != 3 ||
				rows[0].RequestCount != 7 || rows[0].SuccessCount != 6 || rows[0].FailureCount != 1 ||
				rows[0].TotalLatencyMS != 70 || rows[0].LatencySampleCount != 1 || !rows[0].LastEventAt.Equal(bucket1) {
				t.Fatalf("history initialization changed existing facts: %+v", rows)
			}
			for _, row := range rows {
				if row.AccountingValidAttempts != 0 || row.CanonicalTotalTokens != 0 || row.CanonicalCompletePromptTokens != 0 || row.CanonicalCompleteCacheReadTokens != 0 || row.CanonicalCompleteOutputTokens != 0 {
					t.Fatalf("fabricated canonical observation: %+v", row)
				}
			}
			for _, column := range []string{"input_tokens", "billable_prompt_tokens", "output_tokens", "reasoning_tokens", "cached_tokens", "cache_read_tokens", "cache_read_observed_input_tokens", "total_tokens", "accounting_invalid_attempts"} {
				present, err := usageRollupColumnExists(db, column)
				if err != nil {
					t.Fatal(err)
				}
				if present {
					t.Fatalf("inactive derived column retained: %s", column)
				}
			}
			if !db.Migrator().HasIndex("usage_rollups_hourly", "uniq_usage_rollups_hourly_dimensions") || !db.Migrator().HasIndex("usage_rollups_hourly", "idx_usage_rollups_hourly_bucket_provider") {
				t.Fatal("rollup indexes were not preserved")
			}
			newBucket := bucket2.Add(time.Hour)
			if err := db.Create(&entities.UsageRollupHourly{
				BucketStart: newBucket, Provider: "p", Model: "m3", AuthType: "oauth", AuthIndex: "a", APIKeyIdentity: "",
				RequestCount: 1, SuccessCount: 1, AccountingValidAttempts: 1, CanonicalTotalTokens: 5, CanonicalInputTokens: 5,
				CanonicalUncachedTokens: 5, LastEventAt: newBucket,
			}).Error; err != nil {
				t.Fatalf("insert rollup with current entity after upgrade: %v", err)
			}
			// A subsequent boot must leave newly ingested canonical data untouched.
			if err := db.Exec(`UPDATE usage_rollups_hourly SET accounting_absent_attempts = 6, accounting_valid_attempts = 1, canonical_total_tokens = 130 WHERE id = 1`).Error; err != nil {
				t.Fatal(err)
			}
			if err := runSchemaMigration(db, migration); err != nil {
				t.Fatal(err)
			}
			var row entities.UsageRollupHourly
			if err := db.First(&row, 1).Error; err != nil {
				t.Fatal(err)
			}
			if row.AccountingAbsentAttempts != 6 || row.AccountingValidAttempts != 1 || row.CanonicalTotalTokens != 130 {
				t.Fatalf("second boot reset canonical data: %+v", row)
			}
			var count int64
			if err := db.Model(&entities.UsageRollupHourly{}).Count(&count).Error; err != nil || count != 3 {
				t.Fatalf("restart changed rollup rows: count=%d err=%v", count, err)
			}
			if err := db.Table("schema_migrations").Where("version = ?", migration.version).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("rollup accounting migration ledger count=%d err=%v", count, err)
			}
		})
	}
}
