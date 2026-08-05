package app

import (
	"testing"
	"time"

	"cpa-usage/internal/poller"
	repodto "cpa-usage/internal/repository/dto"
)

func int64Ptr(value int64) *int64 {
	return &value
}

func TestBuildMetricsSnapshotIncludesUptimeAndRunnerStates(t *testing.T) {
	startedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	now := startedAt.Add(90 * time.Second)
	covered := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	backupAt := time.Date(2026, 8, 5, 4, 0, 0, 0, time.UTC)
	processedAt := time.Date(2026, 8, 5, 3, 0, 0, 0, time.UTC)

	snapshot := buildMetricsSnapshot(metricsSnapshotInput{
		startedAt: startedAt,
		now:       now,
		pollerStatus: &poller.Status{
			Running:     true,
			SyncRunning: false,
			LastStatus:  "completed",
		},
		rollupStatus: &repodto.RollupBackfillStatus{
			Status:             repodto.RollupBackfillStatusRunning,
			CoveredBucketStart: &covered,
		},
		backupLastAt:           backupAt,
		inboxPending:           int64Ptr(12),
		eventsProcessedTotal:   340,
		eventsProcessedBatches: 8,
		eventsLastProcessedAt:  processedAt,
		eventsRatePerMinute:    12.5,
	})

	if snapshot["uptime_seconds"] != int64(90) {
		t.Fatalf("expected uptime_seconds 90, got %v", snapshot["uptime_seconds"])
	}
	if snapshot["poller_running"] != true {
		t.Fatalf("expected poller_running true, got %v", snapshot["poller_running"])
	}
	if snapshot["poller_sync_running"] != false {
		t.Fatalf("expected poller_sync_running false, got %v", snapshot["poller_sync_running"])
	}
	if snapshot["poller_last_status"] != "completed" {
		t.Fatalf("expected poller_last_status completed, got %v", snapshot["poller_last_status"])
	}
	if snapshot["rollup_backfill_status"] != repodto.RollupBackfillStatusRunning {
		t.Fatalf("expected rollup_backfill_status running, got %v", snapshot["rollup_backfill_status"])
	}
	if snapshot["last_backup_at"] != backupAt.UTC() {
		t.Fatalf("expected last_backup_at %s, got %v", backupAt.UTC(), snapshot["last_backup_at"])
	}
	if snapshot["redis_inbox_pending"] != int64(12) {
		t.Fatalf("expected redis_inbox_pending 12, got %v", snapshot["redis_inbox_pending"])
	}
	if snapshot["redis_events_processed_total"] != int64(340) {
		t.Fatalf("expected redis_events_processed_total 340, got %v", snapshot["redis_events_processed_total"])
	}
	if snapshot["redis_events_processed_batches_total"] != int64(8) {
		t.Fatalf("expected redis_events_processed_batches_total 8, got %v", snapshot["redis_events_processed_batches_total"])
	}
	if snapshot["redis_events_processing_rate_per_minute"] != 12.5 {
		t.Fatalf("expected redis_events_processing_rate_per_minute 12.5, got %v", snapshot["redis_events_processing_rate_per_minute"])
	}
}

func TestBuildMetricsSnapshotOmitsUnavailableStates(t *testing.T) {
	startedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)

	snapshot := buildMetricsSnapshot(metricsSnapshotInput{
		startedAt: startedAt,
		now:       startedAt,
	})

	for _, key := range []string{"poller_running", "poller_sync_running", "poller_last_status", "rollup_backfill_status", "last_backup_at", "redis_events_last_processed_at", "redis_events_processing_rate_per_minute"} {
		if _, exists := snapshot[key]; exists {
			t.Fatalf("expected %q to be omitted when its source is unavailable", key)
		}
	}
	for _, key := range []string{"redis_events_processed_total", "redis_events_processed_batches_total"} {
		if _, exists := snapshot[key]; !exists {
			t.Fatalf("expected %q to always be present", key)
		}
	}
}

func TestBuildMetricsSnapshotMarksDatabaseUnavailable(t *testing.T) {
	startedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)

	snapshot := buildMetricsSnapshot(metricsSnapshotInput{
		startedAt:     startedAt,
		now:           startedAt,
		dbUnavailable: true,
	})

	if snapshot["db_unavailable"] != true {
		t.Fatalf("expected db_unavailable true, got %v", snapshot["db_unavailable"])
	}
	for _, key := range []string{"redis_inbox_pending", "rollup_backfill_status"} {
		if _, exists := snapshot[key]; exists {
			t.Fatalf("expected %q to be omitted when the database read failed", key)
		}
	}
}

func TestEventsPerMinuteComputesDeltaRateBetweenSamples(t *testing.T) {
	previousAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	now := previousAt.Add(2 * time.Minute)

	if rate := eventsPerMinute(previousAt, 100, now, 140); rate != 20 {
		t.Fatalf("expected 20 events/min, got %v", rate)
	}
	if rate := eventsPerMinute(previousAt, 100, previousAt, 100); rate != 0 {
		t.Fatalf("expected 0 when no time elapsed, got %v", rate)
	}
	if rate := eventsPerMinute(time.Time{}, 100, now, 140); rate != 0 {
		t.Fatalf("expected 0 on first sample, got %v", rate)
	}
	if rate := eventsPerMinute(previousAt, 100, now, 80); rate != 0 {
		t.Fatalf("expected 0 when the counter resets, got %v", rate)
	}
}
