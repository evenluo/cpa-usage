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
