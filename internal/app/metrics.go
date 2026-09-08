package app

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	"cpa-usage/internal/poller"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

type metricsSnapshotInput struct {
	startedAt              time.Time
	now                    time.Time
	pollerStatus           *poller.Status
	rollupStatus           *repodto.RollupBackfillStatus
	backupLastAt           time.Time
	inboxPending           *int64
	inboxOldestAgeSeconds  *float64
	eventsProcessedTotal   *int64
	eventsProcessedBatches *int64
	eventsLastProcessedAt  time.Time
	eventsRatePerMinute    *float64
	dbUnavailable          bool
	database               databaseMetricsSnapshot
	runtime                runtimeMetricsSnapshot
	httpRequests           httpRequestMetricsSnapshot
}

type databaseMetricsSnapshot struct {
	Status  string                         `json:"status"`
	Pool    *databasePoolMetricsSnapshot   `json:"pool,omitempty"`
	Storage databaseStorageMetricsSnapshot `json:"storage"`
}

type databasePoolMetricsSnapshot struct {
	OpenConnections          int     `json:"open_connections"`
	InUseConnections         int     `json:"in_use_connections"`
	WaitCountTotal           int64   `json:"wait_count_total"`
	WaitDurationSecondsTotal float64 `json:"wait_duration_seconds_total"`
}

type databaseStorageMetricsSnapshot struct {
	Status        string `json:"status"`
	DatabaseBytes *int64 `json:"database_bytes,omitempty"`
	WALBytes      *int64 `json:"wal_bytes,omitempty"`
}

type runtimeMetricsSnapshot struct {
	Status       string                     `json:"status"`
	Measurements *runtimeMetricMeasurements `json:"measurements,omitempty"`
}

type runtimeMetricMeasurements struct {
	AllocBytesTotal     uint64  `json:"alloc_bytes_total"`
	HeapAllocBytes      uint64  `json:"heap_alloc_bytes"`
	GCCyclesTotal       uint32  `json:"gc_cycles_total"`
	GCPauseSecondsTotal float64 `json:"gc_pause_seconds_total"`
}

// buildMetricsSnapshot 输出与 /healthz 同级公开的运行时快照：
// 只包含聚合数字与状态字符串，不含请求明细或身份信息。
func buildMetricsSnapshot(input metricsSnapshotInput) map[string]any {
	snapshot := map[string]any{
		"uptime_seconds": int64(input.now.Sub(input.startedAt).Seconds()),
	}
	if input.pollerStatus != nil {
		snapshot["poller_running"] = input.pollerStatus.Running
		snapshot["poller_sync_running"] = input.pollerStatus.SyncRunning
		snapshot["poller_last_status"] = input.pollerStatus.LastStatus
	}
	if input.rollupStatus != nil {
		status := repodto.NormalizeRollupBackfillStatus(*input.rollupStatus)
		snapshot["rollup_backfill_status"] = status.Status
		if status.CoveredBucketStart != nil && !status.CoveredBucketStart.IsZero() {
			snapshot["rollup_backfill_covered_bucket_start"] = status.CoveredBucketStart.UTC()
		}
	}
	if !input.backupLastAt.IsZero() {
		snapshot["last_backup_at"] = input.backupLastAt.UTC()
	}
	if input.inboxPending != nil {
		snapshot["redis_inbox_pending"] = *input.inboxPending
	}
	oldestAgeStatus := "unavailable"
	if input.inboxPending != nil && *input.inboxPending == 0 {
		oldestAgeStatus = "empty"
	}
	if input.inboxOldestAgeSeconds != nil {
		oldestAgeStatus = "available"
		snapshot["redis_inbox_oldest_pending_age_seconds"] = *input.inboxOldestAgeSeconds
	}
	snapshot["redis_inbox_oldest_pending_age_status"] = oldestAgeStatus
	if input.dbUnavailable {
		snapshot["db_unavailable"] = true
	}
	if input.database.Status == "" {
		input.database.Status = "unavailable"
	}
	if input.database.Storage.Status == "" {
		input.database.Storage.Status = "unavailable"
	}
	snapshot["database"] = input.database
	if input.runtime.Status == "" {
		input.runtime.Status = "unavailable"
	}
	snapshot["runtime"] = input.runtime
	if input.httpRequests.Status == "" {
		snapshot["http_requests"] = map[string]any{"status": "unavailable"}
	} else {
		snapshot["http_requests"] = input.httpRequests
	}
	if input.eventsProcessedTotal != nil {
		snapshot["redis_events_processed_total"] = *input.eventsProcessedTotal
	}
	if input.eventsProcessedBatches != nil {
		snapshot["redis_events_processed_batches_total"] = *input.eventsProcessedBatches
	}
	if !input.eventsLastProcessedAt.IsZero() {
		snapshot["redis_events_last_processed_at"] = input.eventsLastProcessedAt.UTC()
	}
	if input.eventsRatePerMinute != nil {
		snapshot["redis_events_processing_rate_per_minute"] = *input.eventsRatePerMinute
	}
	return snapshot
}

// eventsPerMinute 用两次快照的累计事件数增量除以间隔分钟计算处理速率。
// 首采样、计数回退或无正间隔时没有可用速率；已完成的相邻采样可以产生真实的 0。
func eventsPerMinute(previousAt time.Time, previousTotal int64, now time.Time, currentTotal int64) (float64, bool) {
	if previousAt.IsZero() || currentTotal < previousTotal {
		return 0, false
	}
	elapsedMinutes := now.Sub(previousAt).Minutes()
	if elapsedMinutes <= 0 {
		return 0, false
	}
	return float64(currentTotal-previousTotal) / elapsedMinutes, true
}

// MetricsSnapshot 实现 api.MetricsProvider：从后台 runner 与 repository 读模型
// 组装公开的运行时快照。事件处理速率按相邻两次抓取的增量计算。
func (a *App) MetricsSnapshot(ctx context.Context) (map[string]any, error) {
	input := metricsSnapshotInput{
		startedAt:    a.startedAt,
		now:          time.Now(),
		runtime:      captureRuntimeMetrics(),
		httpRequests: a.httpMetrics.snapshot(),
	}
	databasePath := ""
	if a.Config != nil {
		databasePath = a.Config.SQLitePath
	}
	input.database = captureDatabaseMetrics(a.DB, databasePath)
	if a.Poller != nil {
		status := a.Poller.Status()
		if a.manualSync != nil {
			// ManualSync owns Last* while retaining the poller's live runner state.
			status = a.manualSync.Status()
		}
		input.pollerStatus = &status
		if provider, ok := a.Poller.(poller.ProcessMetricsProvider); ok {
			metrics := provider.ProcessMetrics()
			input.eventsProcessedTotal = &metrics.EventsTotal
			input.eventsProcessedBatches = &metrics.BatchesTotal
			input.eventsLastProcessedAt = metrics.LastProcessedAt
		}
	}
	status, err := a.rollupBackfillReader.GetRollupBackfillStatus(ctx)
	if err != nil {
		input.dbUnavailable = true
	} else {
		input.rollupStatus = &status
	}
	if a.BackupMaintenance != nil {
		input.backupLastAt = a.BackupMaintenance.LastBackupAt()
	}
	if a.DB != nil {
		backlog, err := repository.ReadRedisUsageInboxBacklog(ctx, a.DB)
		if err != nil {
			input.dbUnavailable = true
		} else {
			input.inboxPending = &backlog.PendingCount
			if backlog.OldestPoppedAt != nil {
				age := input.now.Sub(*backlog.OldestPoppedAt).Seconds()
				if age < 0 {
					age = 0
				}
				input.inboxOldestAgeSeconds = &age
			}
		}
	}

	a.metricsMu.Lock()
	if input.eventsProcessedTotal != nil {
		rate, rateAvailable := eventsPerMinute(
			a.lastMetricsSampleAt,
			a.lastMetricsEventsTotal,
			input.now,
			*input.eventsProcessedTotal,
		)
		if rateAvailable {
			input.eventsRatePerMinute = &rate
		}
		a.lastMetricsSampleAt = input.now
		a.lastMetricsEventsTotal = *input.eventsProcessedTotal
	}
	a.metricsMu.Unlock()

	return buildMetricsSnapshot(input), nil
}

func captureDatabaseMetrics(db *gorm.DB, sqlitePath string) databaseMetricsSnapshot {
	metrics := databaseMetricsSnapshot{
		Status:  "unavailable",
		Storage: captureDatabaseStorageMetrics(sqlitePath),
	}
	if db == nil {
		return metrics
	}
	sqlDB, err := db.DB()
	if err != nil {
		return metrics
	}
	stats := sqlDB.Stats()
	metrics.Status = "available"
	metrics.Pool = &databasePoolMetricsSnapshot{
		OpenConnections:          stats.OpenConnections,
		InUseConnections:         stats.InUse,
		WaitCountTotal:           stats.WaitCount,
		WaitDurationSecondsTotal: stats.WaitDuration.Seconds(),
	}
	return metrics
}

func captureDatabaseStorageMetrics(sqlitePath string) databaseStorageMetricsSnapshot {
	metrics := databaseStorageMetricsSnapshot{Status: "unavailable"}
	path := strings.TrimSpace(sqlitePath)
	if before, _, ok := strings.Cut(path, "?"); ok {
		path = before
	}
	if path == "" || path == ":memory:" {
		return metrics
	}
	databaseInfo, err := os.Stat(path)
	if err != nil || !databaseInfo.Mode().IsRegular() {
		return metrics
	}
	databaseBytes := databaseInfo.Size()

	walBytes := int64(0)
	walInfo, err := os.Stat(path + "-wal")
	if err == nil {
		if !walInfo.Mode().IsRegular() {
			return metrics
		}
		walBytes = walInfo.Size()
	} else if !os.IsNotExist(err) {
		return metrics
	}

	metrics.Status = "available"
	metrics.DatabaseBytes = &databaseBytes
	metrics.WALBytes = &walBytes
	return metrics
}

func captureRuntimeMetrics() runtimeMetricsSnapshot {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return runtimeMetricsSnapshot{
		Status: "available",
		Measurements: &runtimeMetricMeasurements{
			AllocBytesTotal:     stats.TotalAlloc,
			HeapAllocBytes:      stats.HeapAlloc,
			GCCyclesTotal:       stats.NumGC,
			GCPauseSecondsTotal: time.Duration(stats.PauseTotalNs).Seconds(),
		},
	}
}
