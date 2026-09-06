package migration

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageRollupAccountingFieldsSchedulesFullHistoricalBackfill(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "accounting-rollup.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)
	if err := db.Exec(`CREATE TABLE usage_rollups_hourly (id integer PRIMARY KEY, request_count integer NOT NULL)`).Error; err != nil {
		t.Fatalf("create legacy rollup table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_events (id integer PRIMARY KEY, timestamp datetime NOT NULL)`).Error; err != nil {
		t.Fatalf("create usage events table: %v", err)
	}
	if err := db.Exec(`CREATE INDEX idx_usage_events_timestamp ON usage_events(timestamp)`).Error; err != nil {
		t.Fatalf("create usage event timestamp index: %v", err)
	}
	earliest := time.Date(2025, 1, 2, 3, 15, 0, 0, time.UTC)
	latest := time.Date(2026, 9, 7, 8, 45, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO usage_events (id, timestamp) VALUES (?, ?), (?, ?)`, 1, earliest, 2, latest).Error; err != nil {
		t.Fatalf("seed usage events: %v", err)
	}
	if err := db.AutoMigrate(&entities.UsageRollupBackfillState{}); err != nil {
		t.Fatalf("create backfill state: %v", err)
	}
	completed := latest.Truncate(time.Hour)
	if err := db.Create(&entities.UsageRollupBackfillState{
		Name: entities.UsageRollupBackfillStateName, Status: "completed",
		TargetBucketStart: &completed, CoveredBucketStart: &completed, CompletedAt: &completed,
	}).Error; err != nil {
		t.Fatalf("seed completed state: %v", err)
	}

	if err := addUsageRollupAccountingFieldsMigration(db); err != nil {
		t.Fatalf("add rollup accounting fields: %v", err)
	}
	for _, column := range []string{"accounting_absent_attempts", "accounting_valid_attempts", "canonical_total_tokens", "canonical_unclassified_tokens"} {
		if !db.Migrator().HasColumn("usage_rollups_hourly", column) {
			t.Fatalf("expected usage_rollups_hourly.%s", column)
		}
	}
	var state entities.UsageRollupBackfillState
	if err := db.Where("name = ?", entities.UsageRollupBackfillStateName).First(&state).Error; err != nil {
		t.Fatalf("read scheduled state: %v", err)
	}
	wantCovered := earliest.Truncate(time.Hour).Add(-time.Hour)
	wantTarget := latest.Truncate(time.Hour)
	if state.Status != entities.UsageRollupBackfillStateStatusPending || state.CoveredBucketStart == nil || !state.CoveredBucketStart.Equal(wantCovered) || state.TargetBucketStart == nil || !state.TargetBucketStart.Equal(wantTarget) {
		t.Fatalf("expected full historical accounting backfill covered=%s target=%s, got %+v", wantCovered, wantTarget, state)
	}
	if state.StartedAt != nil || state.CompletedAt != nil || state.FailedAt != nil || state.LastError != "" {
		t.Fatalf("expected pending lifecycle metadata to reset, got %+v", state)
	}

	var plan []struct{ Detail string }
	if err := db.Raw(`EXPLAIN QUERY PLAN SELECT
		strftime('%Y-%m-%dT%H:00:00Z', (SELECT MIN(timestamp) FROM usage_events)),
		strftime('%Y-%m-%dT%H:00:00Z', (SELECT MAX(timestamp) FROM usage_events))`).Scan(&plan).Error; err != nil {
		t.Fatalf("explain accounting migration bounds query: %v", err)
	}
	var detail strings.Builder
	for _, step := range plan {
		detail.WriteString(step.Detail)
		detail.WriteByte('\n')
	}
	if strings.Count(detail.String(), "USING COVERING INDEX idx_usage_events_timestamp") < 2 || strings.Contains(detail.String(), "SCAN usage_events") {
		t.Fatalf("expected both extrema subqueries to use the timestamp index, got:\n%s", detail.String())
	}
}
