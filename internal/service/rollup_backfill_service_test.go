package service

import (
	"context"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func TestUsageRollupBackfillRunnerUsesConfiguredBatchAndIdle(t *testing.T) {
	db := openRollupBackfillServiceTestDatabase(t)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "rollup-service-09", Provider: "OpenAI", Model: "model", Timestamp: time.Date(2026, 7, 7, 9, 15, 0, 0, time.UTC), TotalTokens: 10},
		{EventKey: "rollup-service-10", Provider: "OpenAI", Model: "model", Timestamp: time.Date(2026, 7, 7, 10, 15, 0, 0, time.UTC), TotalTokens: 20},
	}); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	runner := NewUsageRollupBackfillRunner(db, UsageRollupBackfillRunnerConfig{
		BatchHours:   1,
		IdleInterval: 3 * time.Second,
		RetryBackoff: 7 * time.Second,
	})
	if runner.batchHours != 1 || runner.idleInterval != 3*time.Second || runner.retryBackoff != 7*time.Second {
		t.Fatalf("expected runner to use configured values, got batch=%d idle=%s retry=%s", runner.batchHours, runner.idleInterval, runner.retryBackoff)
	}
	runner.now = func() time.Time {
		return time.Date(2026, 7, 7, 11, 30, 0, 0, time.UTC)
	}
	var sleepDurations []time.Duration
	runner.sleep = func(_ context.Context, duration time.Duration) bool {
		sleepDurations = append(sleepDurations, duration)
		return true
	}

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(sleepDurations) != 1 || sleepDurations[0] != 3*time.Second {
		t.Fatalf("expected one configured idle sleep between two successful batches, got %v", sleepDurations)
	}

	status, err := repository.GetUsageRollupBackfillStatus(db)
	if err != nil {
		t.Fatalf("load backfill status: %v", err)
	}
	if status.Status != repodto.RollupBackfillStatusCompleted {
		t.Fatalf("expected completed backfill status, got %+v", status)
	}
}

func openRollupBackfillServiceTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := repository.OpenDatabase(config.Config{SQLitePath: t.TempDir() + "/app.db"})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("load sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
