package repository

import (
	"testing"
	"time"

	"cpa-usage/internal/entities"
	repodto "cpa-usage/internal/repository/dto"
)

func TestGetUsageRollupBackfillStatusDefaultsToPending(t *testing.T) {
	db := openTestDatabase(t)

	status, err := GetUsageRollupBackfillStatus(db)
	if err != nil {
		t.Fatalf("GetUsageRollupBackfillStatus returned error: %v", err)
	}

	if status.Status != repodto.RollupBackfillStatusPending {
		t.Fatalf("expected pending status for empty database, got %+v", status)
	}
	if status.TargetBucketStart != nil || status.CoveredBucketStart != nil || status.StartedAt != nil || status.CompletedAt != nil || status.FailedAt != nil || status.LastError != "" {
		t.Fatalf("expected empty pending status details, got %+v", status)
	}
	var count int64
	if err := db.Model(&entities.UsageRollupBackfillState{}).Where("name = ? AND status = ?", entities.UsageRollupBackfillStateName, entities.UsageRollupBackfillStateStatusPending).Count(&count).Error; err != nil {
		t.Fatalf("count persisted backfill state: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one persisted pending backfill state row, got %d", count)
	}
}

func TestSaveUsageRollupBackfillStatusPersistsProgress(t *testing.T) {
	db := openTestDatabase(t)
	target := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	covered := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	started := time.Date(2026, 7, 7, 1, 0, 0, 0, time.UTC)

	err := SaveUsageRollupBackfillStatus(db, repodto.RollupBackfillStatus{
		Status:             repodto.RollupBackfillStatusRunning,
		TargetBucketStart:  &target,
		CoveredBucketStart: &covered,
		StartedAt:          &started,
	})
	if err != nil {
		t.Fatalf("SaveUsageRollupBackfillStatus returned error: %v", err)
	}

	status, err := GetUsageRollupBackfillStatus(db)
	if err != nil {
		t.Fatalf("GetUsageRollupBackfillStatus returned error: %v", err)
	}

	if status.Status != repodto.RollupBackfillStatusRunning {
		t.Fatalf("expected running status, got %+v", status)
	}
	if !status.TargetBucketStart.Equal(target) || !status.CoveredBucketStart.Equal(covered) || !status.StartedAt.Equal(started) {
		t.Fatalf("expected bucket progress to round-trip, got %+v", status)
	}

	failed := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	err = SaveUsageRollupBackfillStatus(db, repodto.RollupBackfillStatus{
		Status:             repodto.RollupBackfillStatusFailed,
		TargetBucketStart:  &target,
		CoveredBucketStart: &covered,
		StartedAt:          &started,
		FailedAt:           &failed,
		LastError:          "disk is full",
	})
	if err != nil {
		t.Fatalf("SaveUsageRollupBackfillStatus update returned error: %v", err)
	}

	status, err = GetUsageRollupBackfillStatus(db)
	if err != nil {
		t.Fatalf("GetUsageRollupBackfillStatus after update returned error: %v", err)
	}
	if status.Status != repodto.RollupBackfillStatusFailed || status.LastError != "disk is full" || !status.FailedAt.Equal(failed) {
		t.Fatalf("expected failed status to replace running row, got %+v", status)
	}
}

func TestBackfillUsageRollupsBatchProgressesAndResumes(t *testing.T) {
	db := openTestDatabase(t)
	events := []entities.UsageEvent{
		{EventKey: "event-09", Provider: "OpenAI", Model: "model", Timestamp: time.Date(2026, 7, 7, 9, 15, 0, 0, time.UTC), TotalTokens: 10},
		{EventKey: "event-10", Provider: "OpenAI", Model: "model", Timestamp: time.Date(2026, 7, 7, 10, 15, 0, 0, time.UTC), TotalTokens: 20},
		{EventKey: "event-12", Provider: "OpenAI", Model: "model", Timestamp: time.Date(2026, 7, 7, 12, 15, 0, 0, time.UTC), TotalTokens: 30},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	if err := db.Where("1 = 1").Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		t.Fatalf("clear ingestion rollups: %v", err)
	}

	now := time.Date(2026, 7, 7, 13, 30, 0, 0, time.UTC)
	first, err := BackfillUsageRollupsBatch(db, now, 2)
	if err != nil {
		t.Fatalf("first BackfillUsageRollupsBatch returned error: %v", err)
	}
	if first.Done || first.Status.Status != repodto.RollupBackfillStatusRunning || first.RebuiltBucketCount != 2 {
		t.Fatalf("expected first bounded batch to remain running, got %+v", first)
	}
	expectedCovered := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	if first.Status.CoveredBucketStart == nil || !first.Status.CoveredBucketStart.Equal(expectedCovered) {
		t.Fatalf("expected first covered bucket %s, got %+v", expectedCovered, first.Status)
	}

	second, err := BackfillUsageRollupsBatch(db, now.Add(time.Minute), 2)
	if err != nil {
		t.Fatalf("second BackfillUsageRollupsBatch returned error: %v", err)
	}
	target := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	if !second.Done || second.Status.Status != repodto.RollupBackfillStatusCompleted || second.Status.CoveredBucketStart == nil || !second.Status.CoveredBucketStart.Equal(target) {
		t.Fatalf("expected second batch to complete target %s, got %+v", target, second)
	}

	var rollups []entities.UsageRollupHourly
	if err := db.Order("bucket_start ASC").Find(&rollups).Error; err != nil {
		t.Fatalf("load rollups: %v", err)
	}
	if len(rollups) != 3 {
		t.Fatalf("expected rollups for three non-empty buckets, got %+v", rollups)
	}
	if rollups[0].TotalTokens != 10 || rollups[1].TotalTokens != 20 || rollups[2].TotalTokens != 30 {
		t.Fatalf("unexpected rollup totals after backfill: %+v", rollups)
	}
}

func TestBackfillUsageRollupsBatchRetriesFailedProgress(t *testing.T) {
	db := openTestDatabase(t)
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "event-12", Provider: "OpenAI", Model: "model", Timestamp: time.Date(2026, 7, 7, 12, 15, 0, 0, time.UTC), TotalTokens: 30},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	if err := db.Where("1 = 1").Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		t.Fatalf("clear ingestion rollups: %v", err)
	}
	target := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	covered := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	failedAt := time.Date(2026, 7, 7, 11, 0, 0, 0, time.UTC)
	if err := SaveUsageRollupBackfillStatus(db, repodto.RollupBackfillStatus{
		Status:             repodto.RollupBackfillStatusFailed,
		TargetBucketStart:  &target,
		CoveredBucketStart: &covered,
		FailedAt:           &failedAt,
		LastError:          "transient failure",
	}); err != nil {
		t.Fatalf("seed failed status: %v", err)
	}

	result, err := BackfillUsageRollupsBatch(db, time.Date(2026, 7, 7, 13, 30, 0, 0, time.UTC), 24)
	if err != nil {
		t.Fatalf("BackfillUsageRollupsBatch returned error: %v", err)
	}
	if !result.Done || result.Status.Status != repodto.RollupBackfillStatusCompleted || result.Status.LastError != "" || result.Status.FailedAt != nil {
		t.Fatalf("expected failed progress to retry and clear failure metadata, got %+v", result.Status)
	}
	var rollup entities.UsageRollupHourly
	if err := db.Where("bucket_start = ?", target).First(&rollup).Error; err != nil {
		t.Fatalf("expected retried target rollup: %v", err)
	}
}
