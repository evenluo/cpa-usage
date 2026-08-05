package app

import (
	"context"
	"time"

	"cpa-usage/internal/poller"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
)

type metricsSnapshotInput struct {
	startedAt              time.Time
	now                    time.Time
	pollerStatus           *poller.Status
	rollupStatus           *repodto.RollupBackfillStatus
	backupLastAt           time.Time
	inboxPending           *int64
	eventsProcessedTotal   int64
	eventsProcessedBatches int64
	eventsLastProcessedAt  time.Time
	eventsRatePerMinute    float64
	dbUnavailable          bool
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
		if !status.CoveredBucketStart.IsZero() {
			snapshot["rollup_backfill_covered_bucket_start"] = status.CoveredBucketStart.UTC()
		}
	}
	if !input.backupLastAt.IsZero() {
		snapshot["last_backup_at"] = input.backupLastAt.UTC()
	}
	if input.inboxPending != nil {
		snapshot["redis_inbox_pending"] = *input.inboxPending
	}
	if input.dbUnavailable {
		snapshot["db_unavailable"] = true
	}
	snapshot["redis_events_processed_total"] = input.eventsProcessedTotal
	snapshot["redis_events_processed_batches_total"] = input.eventsProcessedBatches
	if !input.eventsLastProcessedAt.IsZero() {
		snapshot["redis_events_last_processed_at"] = input.eventsLastProcessedAt.UTC()
	}
	if input.eventsRatePerMinute > 0 {
		snapshot["redis_events_processing_rate_per_minute"] = input.eventsRatePerMinute
	}
	return snapshot
}

// eventsPerMinute 用两次快照的累计事件数增量除以间隔分钟计算处理速率；
// 首采样或计数回退时返回 0。
func eventsPerMinute(previousAt time.Time, previousTotal int64, now time.Time, currentTotal int64) float64 {
	if previousAt.IsZero() || currentTotal < previousTotal {
		return 0
	}
	elapsedMinutes := now.Sub(previousAt).Minutes()
	if elapsedMinutes <= 0 {
		return 0
	}
	return float64(currentTotal-previousTotal) / elapsedMinutes
}

// MetricsSnapshot 实现 api.MetricsProvider：从后台 runner 与 repository 读模型
// 组装公开的运行时快照。事件处理速率按相邻两次抓取的增量计算。
func (a *App) MetricsSnapshot(ctx context.Context) (map[string]any, error) {
	input := metricsSnapshotInput{
		startedAt: a.startedAt,
		now:       time.Now(),
	}
	if a.Poller != nil {
		status := a.Poller.Status()
		input.pollerStatus = &status
		if provider, ok := a.Poller.(poller.ProcessMetricsProvider); ok {
			metrics := provider.ProcessMetrics()
			input.eventsProcessedTotal = metrics.EventsTotal
			input.eventsProcessedBatches = metrics.BatchesTotal
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
		pending, err := repository.CountPendingRedisUsageInbox(a.DB)
		if err != nil {
			input.dbUnavailable = true
		} else {
			input.inboxPending = &pending
		}
	}

	a.metricsMu.Lock()
	input.eventsRatePerMinute = eventsPerMinute(
		a.lastMetricsSampleAt,
		a.lastMetricsEventsTotal,
		input.now,
		input.eventsProcessedTotal,
	)
	a.lastMetricsSampleAt = input.now
	a.lastMetricsEventsTotal = input.eventsProcessedTotal
	a.metricsMu.Unlock()

	return buildMetricsSnapshot(input), nil
}
