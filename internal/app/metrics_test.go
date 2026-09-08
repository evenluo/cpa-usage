package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		eventsProcessedTotal:   int64Ptr(340),
		eventsProcessedBatches: int64Ptr(8),
		eventsLastProcessedAt:  processedAt,
		eventsRatePerMinute:    float64Ptr(12.5),
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
		if _, exists := snapshot[key]; exists {
			t.Fatalf("expected %q to be omitted when processing metrics are unavailable", key)
		}
	}
}

func TestBuildMetricsSnapshotOmitsMissingRollupCoverageTimestamp(t *testing.T) {
	for _, status := range []string{
		repodto.RollupBackfillStatusPending,
		repodto.RollupBackfillStatusRunning,
		repodto.RollupBackfillStatusFailed,
	} {
		t.Run(status, func(t *testing.T) {
			snapshot := buildMetricsSnapshot(metricsSnapshotInput{
				startedAt:    time.Now(),
				now:          time.Now(),
				rollupStatus: &repodto.RollupBackfillStatus{Status: status},
			})
			if snapshot["rollup_backfill_status"] != status {
				t.Fatalf("expected status %q, got %v", status, snapshot["rollup_backfill_status"])
			}
			if _, exists := snapshot["rollup_backfill_covered_bucket_start"]; exists {
				t.Fatal("expected missing coverage timestamp to be omitted")
			}
		})
	}
}

func TestMetricsRouteServesPendingRollupWithoutCoverageTimestamp(t *testing.T) {
	application, err := NewWithConfig(testAppConfig(t))
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	defer application.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	application.Router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected /metrics 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var snapshot map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if snapshot["rollup_backfill_status"] != repodto.RollupBackfillStatusPending {
		t.Fatalf("expected pending rollup status, got %v", snapshot["rollup_backfill_status"])
	}
	if _, exists := snapshot["rollup_backfill_covered_bucket_start"]; exists {
		t.Fatal("expected uncovered pending status to omit coverage timestamp")
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

func float64Ptr(value float64) *float64 {
	return &value
}

func TestEventsPerMinuteComputesDeltaRateBetweenSamples(t *testing.T) {
	previousAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	now := previousAt.Add(2 * time.Minute)

	if rate, available := eventsPerMinute(previousAt, 100, now, 140); !available || rate != 20 {
		t.Fatalf("expected 20 events/min, got %v", rate)
	}
	if _, available := eventsPerMinute(previousAt, 100, previousAt, 100); available {
		t.Fatal("expected no rate when no time elapsed")
	}
	if _, available := eventsPerMinute(time.Time{}, 100, now, 140); available {
		t.Fatal("expected no rate on first sample")
	}
	if _, available := eventsPerMinute(previousAt, 100, now, 80); available {
		t.Fatal("expected no rate when the counter resets")
	}
}

func TestBuildMetricsSnapshotPreservesObservedZeroRate(t *testing.T) {
	zero := float64(0)
	snapshot := buildMetricsSnapshot(metricsSnapshotInput{
		startedAt:              time.Now(),
		now:                    time.Now(),
		eventsProcessedTotal:   int64Ptr(0),
		eventsProcessedBatches: int64Ptr(0),
		eventsRatePerMinute:    &zero,
	})
	if snapshot["redis_events_processing_rate_per_minute"] != zero {
		t.Fatalf("expected observed zero rate, got %v", snapshot["redis_events_processing_rate_per_minute"])
	}
}

func TestBuildMetricsSnapshotOmitsUnobservedProcessedVolume(t *testing.T) {
	snapshot := buildMetricsSnapshot(metricsSnapshotInput{
		startedAt: time.Now(),
		now:       time.Now(),
	})
	for _, key := range []string{"redis_events_processed_total", "redis_events_processed_batches_total"} {
		if _, exists := snapshot[key]; exists {
			t.Fatalf("expected %q to be omitted without a process metrics provider", key)
		}
	}
}

func TestBuildMetricsSnapshotDistinguishesInboxAgeAvailability(t *testing.T) {
	pending := int64(2)
	age := 17.25
	snapshot := buildMetricsSnapshot(metricsSnapshotInput{
		startedAt:             time.Now(),
		now:                   time.Now(),
		inboxPending:          &pending,
		inboxOldestAgeSeconds: &age,
	})
	if snapshot["redis_inbox_oldest_pending_age_status"] != "available" || snapshot["redis_inbox_oldest_pending_age_seconds"] != age {
		t.Fatalf("expected available inbox age, got %+v", snapshot)
	}

	empty := int64(0)
	snapshot = buildMetricsSnapshot(metricsSnapshotInput{
		startedAt:    time.Now(),
		now:          time.Now(),
		inboxPending: &empty,
	})
	if snapshot["redis_inbox_oldest_pending_age_status"] != "empty" {
		t.Fatalf("expected empty inbox age status, got %+v", snapshot)
	}
	if _, exists := snapshot["redis_inbox_oldest_pending_age_seconds"]; exists {
		t.Fatal("expected empty inbox to omit an age value")
	}
}

func TestCaptureDatabaseStorageMetricsReportsSizesWithoutPath(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "customer-secret")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatalf("create database directory: %v", err)
	}
	databasePath := filepath.Join(directory, "account-secret.db")
	if err := os.WriteFile(databasePath, []byte("database"), 0o600); err != nil {
		t.Fatalf("write database fixture: %v", err)
	}
	if err := os.WriteFile(databasePath+"-wal", []byte("wal"), 0o600); err != nil {
		t.Fatalf("write WAL fixture: %v", err)
	}

	metrics := captureDatabaseStorageMetrics(databasePath + "?_busy_timeout=5000")
	if metrics.Status != "available" || metrics.DatabaseBytes == nil || *metrics.DatabaseBytes != 8 || metrics.WALBytes == nil || *metrics.WALBytes != 3 {
		t.Fatalf("unexpected storage metrics: %+v", metrics)
	}
	encoded, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("marshal storage metrics: %v", err)
	}
	for _, secret := range []string{"customer-secret", "account-secret", databasePath} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("storage metrics leaked path content %q: %s", secret, encoded)
		}
	}
}

func TestCaptureDatabaseStorageMetricsTreatsMissingWALAsObservedZero(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "app.db")
	if err := os.WriteFile(databasePath, []byte("db"), 0o600); err != nil {
		t.Fatalf("write database fixture: %v", err)
	}

	metrics := captureDatabaseStorageMetrics(databasePath)
	if metrics.Status != "available" || metrics.WALBytes == nil || *metrics.WALBytes != 0 {
		t.Fatalf("expected absent WAL to be observed as zero bytes, got %+v", metrics)
	}
}

func TestMetricsRouteIncludesPoolRuntimeStorageAndHTTPRequestMetrics(t *testing.T) {
	application, err := NewWithConfig(testAppConfig(t))
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	defer application.Close()

	application.Router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/ping?api_key=secret", nil))
	recorder := httptest.NewRecorder()
	application.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected /metrics 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "api_key") {
		t.Fatalf("metrics response leaked request query: %s", recorder.Body.String())
	}

	var snapshot map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	database, ok := snapshot["database"].(map[string]any)
	if !ok || database["status"] != "available" {
		t.Fatalf("expected available database metrics, got %v", snapshot["database"])
	}
	pool, ok := database["pool"].(map[string]any)
	if !ok || pool["open_connections"] == nil || pool["in_use_connections"] == nil || pool["wait_count_total"] == nil || pool["wait_duration_seconds_total"] == nil {
		t.Fatalf("expected complete database pool stats, got %v", database)
	}
	storage, ok := database["storage"].(map[string]any)
	if !ok || storage["status"] != "available" || storage["database_bytes"] == nil || storage["wal_bytes"] == nil {
		t.Fatalf("expected available database storage stats, got %v", database)
	}
	runtimeMetrics, ok := snapshot["runtime"].(map[string]any)
	if !ok || runtimeMetrics["status"] != "available" || runtimeMetrics["measurements"] == nil {
		t.Fatalf("expected available runtime metrics, got %v", snapshot["runtime"])
	}
	httpMetrics, ok := snapshot["http_requests"].(map[string]any)
	if !ok || httpMetrics["status"] != "available" {
		t.Fatalf("expected available HTTP metrics, got %v", snapshot["http_requests"])
	}
	routes, ok := httpMetrics["routes"].([]any)
	if !ok || len(routes) != 1 || routes[0].(map[string]any)["route"] != "/api/v1/ping" {
		t.Fatalf("expected ping route-template metrics, got %v", httpMetrics)
	}
}
