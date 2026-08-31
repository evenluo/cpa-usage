package migration

import (
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageRollupCacheReadFieldsMigrationSchedulesOnlyExactFactBuckets(t *testing.T) {
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
	if err := db.Exec(`CREATE TABLE usage_events (id integer PRIMARY KEY, timestamp datetime NOT NULL, input_tokens integer NOT NULL, cache_read_tokens integer)`).Error; err != nil {
		t.Fatalf("create usage events table: %v", err)
	}
	old := time.Date(2020, 1, 2, 3, 15, 0, 0, time.UTC)
	earliestExact := time.Date(2026, 8, 31, 8, 45, 0, 0, time.UTC)
	latestExact := time.Date(2026, 8, 31, 10, 5, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO usage_events (id, timestamp, input_tokens, cache_read_tokens) VALUES
		(?, ?, ?, ?), (?, ?, ?, NULL), (?, ?, ?, ?), (?, ?, ?, ?), (?, ?, ?, ?)`,
		1, old.Add(-time.Hour), 0, 1,
		2, old, 100,
		3, earliestExact, 100, 0,
		4, latestExact, 100, 25,
		5, latestExact.Add(time.Hour), 100, 101).Error; err != nil {
		t.Fatalf("seed usage events: %v", err)
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

	var state entities.UsageRollupBackfillState
	if err := db.Where("name = ?", entities.UsageRollupBackfillStateName).First(&state).Error; err != nil {
		t.Fatalf("read scheduled backfill state: %v", err)
	}
	wantTarget := latestExact.Truncate(time.Hour)
	wantCovered := earliestExact.Truncate(time.Hour).Add(-time.Hour)
	if state.Status != entities.UsageRollupBackfillStateStatusPending || state.TargetBucketStart == nil || !state.TargetBucketStart.Equal(wantTarget) || state.CoveredBucketStart == nil || !state.CoveredBucketStart.Equal(wantCovered) {
		t.Fatalf("expected exact-fact backfill bounds covered=%s target=%s, got %+v", wantCovered, wantTarget, state)
	}
	if state.StartedAt != nil || state.CompletedAt != nil || state.FailedAt != nil || state.LastError != "" {
		t.Fatalf("expected fresh pending lifecycle metadata, got %+v", state)
	}
}

func TestAddUsageRollupCacheReadFieldsMigrationUnionsIncompleteBackfillRange(t *testing.T) {
	for _, existingStatus := range []string{entities.UsageRollupBackfillStateStatusPending, "failed"} {
		t.Run(existingStatus, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "incomplete-rollup.db"))), &gorm.Config{})
			if err != nil {
				t.Fatalf("open database: %v", err)
			}
			defer closeOpenedDatabase(t, db)

			if err := db.Exec(`CREATE TABLE usage_rollups_hourly (id integer PRIMARY KEY, input_tokens integer NOT NULL, cached_tokens integer NOT NULL)`).Error; err != nil {
				t.Fatalf("create legacy rollup table: %v", err)
			}
			if err := db.Exec(`CREATE TABLE usage_events (id integer PRIMARY KEY, timestamp datetime NOT NULL, input_tokens integer NOT NULL, cache_read_tokens integer)`).Error; err != nil {
				t.Fatalf("create usage events table: %v", err)
			}
			existingCovered := time.Date(2026, 8, 30, 5, 0, 0, 0, time.UTC)
			earliestExact := time.Date(2026, 8, 30, 3, 45, 0, 0, time.UTC)
			existingTarget := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
			latestExact := time.Date(2026, 8, 31, 14, 5, 0, 0, time.UTC)
			if err := db.Exec(`INSERT INTO usage_events (id, timestamp, input_tokens, cache_read_tokens) VALUES (?, ?, ?, NULL), (?, ?, ?, ?), (?, ?, ?, ?)`,
				1, existingCovered.Add(time.Minute), 100, 2, earliestExact, 100, 0, 3, latestExact, 100, 25).Error; err != nil {
				t.Fatalf("seed usage events: %v", err)
			}
			if err := db.AutoMigrate(&entities.UsageRollupBackfillState{}); err != nil {
				t.Fatalf("create backfill state: %v", err)
			}
			started := existingCovered.Add(-time.Hour)
			if err := db.Create(&entities.UsageRollupBackfillState{
				Name:               entities.UsageRollupBackfillStateName,
				Status:             existingStatus,
				TargetBucketStart:  &existingTarget,
				CoveredBucketStart: &existingCovered,
				StartedAt:          &started,
				FailedAt:           &started,
				LastError:          "stale",
			}).Error; err != nil {
				t.Fatalf("seed %s backfill state: %v", existingStatus, err)
			}

			if err := addUsageRollupCacheReadFieldsMigration(db); err != nil {
				t.Fatalf("add rollup cache-read fields: %v", err)
			}
			var state entities.UsageRollupBackfillState
			if err := db.Where("name = ?", entities.UsageRollupBackfillStateName).First(&state).Error; err != nil {
				t.Fatalf("read unioned %s state: %v", existingStatus, err)
			}
			wantCovered := earliestExact.Truncate(time.Hour).Add(-time.Hour)
			wantTarget := latestExact.Truncate(time.Hour)
			if state.Status != entities.UsageRollupBackfillStateStatusPending || state.TargetBucketStart == nil || !state.TargetBucketStart.Equal(wantTarget) || state.CoveredBucketStart == nil || !state.CoveredBucketStart.Equal(wantCovered) {
				t.Fatalf("expected unioned bounds covered=%s target=%s, got %+v", wantCovered, wantTarget, state)
			}
			if state.StartedAt != nil || state.CompletedAt != nil || state.FailedAt != nil || state.LastError != "" {
				t.Fatalf("expected pending lifecycle metadata to reset, got %+v", state)
			}
		})
	}
}

func TestAddUsageRollupCacheReadFieldsMigrationPreservesCompletedStateWithoutValidFacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy-rollup.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := db.Exec(`CREATE TABLE usage_rollups_hourly (id integer PRIMARY KEY, input_tokens integer NOT NULL, cached_tokens integer NOT NULL)`).Error; err != nil {
		t.Fatalf("create legacy rollup table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_events (id integer PRIMARY KEY, timestamp datetime NOT NULL, input_tokens integer NOT NULL, cache_read_tokens integer)`).Error; err != nil {
		t.Fatalf("create usage events table: %v", err)
	}
	invalidAt := time.Date(2020, 1, 2, 3, 15, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO usage_events (id, timestamp, input_tokens, cache_read_tokens) VALUES (?, ?, ?, ?), (?, ?, ?, ?), (?, ?, ?, ?), (?, ?, ?, NULL)`,
		1, invalidAt, 0, 1,
		2, invalidAt.Add(time.Hour), 100, 101,
		3, invalidAt.Add(2*time.Hour), 100, -1,
		4, invalidAt.Add(3*time.Hour), 100).Error; err != nil {
		t.Fatalf("seed usage events without valid cache facts: %v", err)
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
	var state entities.UsageRollupBackfillState
	if err := db.Where("name = ?", entities.UsageRollupBackfillStateName).First(&state).Error; err != nil {
		t.Fatalf("read preserved backfill state: %v", err)
	}
	if state.Status != "completed" || state.TargetBucketStart == nil || !state.TargetBucketStart.Equal(completed) || state.CoveredBucketStart == nil || !state.CoveredBucketStart.Equal(completed) || state.CompletedAt == nil || !state.CompletedAt.Equal(completed) {
		t.Fatalf("expected completed state to remain unchanged, got %+v", state)
	}
}
