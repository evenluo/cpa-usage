package migration

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageRollupCacheReadFieldsMigrationPreservesRowsAndResetsBackfill(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy-rollup.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := db.Exec(`CREATE TABLE usage_rollups_hourly (id integer PRIMARY KEY, input_tokens integer NOT NULL, cached_tokens integer NOT NULL)`).Error; err != nil {
		t.Fatalf("create legacy rollup table: %v", err)
	}
	if err := db.Exec(`INSERT INTO usage_rollups_hourly (id, input_tokens, cached_tokens) VALUES (1, 100, 20)`).Error; err != nil {
		t.Fatalf("seed legacy rollup: %v", err)
	}
	if err := db.AutoMigrate(&entities.UsageRollupBackfillState{}); err != nil {
		t.Fatalf("create backfill state: %v", err)
	}
	completed := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	if err := db.Create(&entities.UsageRollupBackfillState{
		Name: entities.UsageRollupBackfillStateName, Status: "completed", TargetBucketStart: &completed, CoveredBucketStart: &completed, CompletedAt: &completed,
	}).Error; err != nil {
		t.Fatalf("seed completed backfill state: %v", err)
	}

	if err := addUsageRollupCacheReadFieldsMigration(db); err != nil {
		t.Fatalf("add rollup cache-read fields: %v", err)
	}
	for _, column := range []string{"cache_read_tokens", "cache_read_observed_input_tokens"} {
		if !db.Migrator().HasColumn("usage_rollups_hourly", column) {
			t.Fatalf("expected usage_rollups_hourly.%s column", column)
		}
	}
	var inputTokens, cachedTokens, cacheReadTokens, observedInputTokens int64
	if err := db.Raw(`SELECT input_tokens, cached_tokens, cache_read_tokens, cache_read_observed_input_tokens FROM usage_rollups_hourly WHERE id = 1`).Row().Scan(&inputTokens, &cachedTokens, &cacheReadTokens, &observedInputTokens); err != nil {
		t.Fatalf("read migrated rollup: %v", err)
	}
	if inputTokens != 100 || cachedTokens != 20 || cacheReadTokens != 0 || observedInputTokens != 0 {
		t.Fatalf("unexpected migrated rollup values: input=%d cached=%d read=%d observed_input=%d", inputTokens, cachedTokens, cacheReadTokens, observedInputTokens)
	}

	var status string
	var target, covered, completedAt sql.NullTime
	if err := db.Raw(`SELECT status, target_bucket_start, covered_bucket_start, completed_at FROM usage_rollup_backfill_states WHERE name = ?`, entities.UsageRollupBackfillStateName).Row().Scan(&status, &target, &covered, &completedAt); err != nil {
		t.Fatalf("read reset backfill state: %v", err)
	}
	if status != entities.UsageRollupBackfillStateStatusPending || target.Valid || covered.Valid || completedAt.Valid {
		t.Fatalf("expected pending backfill reset, got status=%q target=%+v covered=%+v completed=%+v", status, target, covered, completedAt)
	}
}
