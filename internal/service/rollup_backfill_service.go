package service

import (
	"context"
	"log/slog"
	"time"

	"cpa-usage/internal/repository"
	"gorm.io/gorm"
)

type UsageRollupBackfillRunnerConfig struct {
	BatchHours   int
	IdleInterval time.Duration
	RetryBackoff time.Duration
}

type UsageRollupBackfillRunner struct {
	db           *gorm.DB
	batchHours   int
	idleInterval time.Duration
	retryBackoff time.Duration
	now          func() time.Time
	sleep        func(context.Context, time.Duration) bool
}

func NewUsageRollupBackfillRunner(db *gorm.DB, cfg UsageRollupBackfillRunnerConfig) *UsageRollupBackfillRunner {
	batchHours := cfg.BatchHours
	if batchHours <= 0 {
		batchHours = repository.DefaultUsageRollupBackfillBatchHours
	}
	idleInterval := cfg.IdleInterval
	if idleInterval <= 0 {
		idleInterval = 2 * time.Second
	}
	retryBackoff := cfg.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = 30 * time.Second
	}
	return &UsageRollupBackfillRunner{db: db, batchHours: batchHours, idleInterval: idleInterval, retryBackoff: retryBackoff, now: time.Now, sleep: sleepContext}
}

func (r *UsageRollupBackfillRunner) Run(ctx context.Context) error {
	if r == nil {
		return nil
	}
	for {
		result, err := repository.BackfillUsageRollupsBatch(r.db, r.now().UTC(), r.batchHours)
		if err != nil {
			slog.Warn("usage rollup backfill batch failed", "error", err)
			if !r.sleep(ctx, r.retryBackoff) {
				return ctx.Err()
			}
			continue
		}
		if result.Done {
			slog.Info("usage rollup backfill completed", "status", result.Status.Status)
			return nil
		}
		if !r.sleep(ctx, r.idleInterval) {
			return ctx.Err()
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
