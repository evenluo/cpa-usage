package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// redisUsageInboxBacklogQuery keeps the metrics read to one SQL statement.
// COUNT scans the compact processable-row index, while the head timestamp is
// fetched from the first indexed inbox ID instead of scanning every popped_at.
const redisUsageInboxBacklogQuery = `
	SELECT
		(
			SELECT COUNT(*)
			FROM redis_usage_inboxes INDEXED BY idx_redis_usage_inboxes_processable_id
			WHERE status = 'pending' OR status = 'process_failed'
		) AS pending_count,
		(
			SELECT popped_at
			FROM redis_usage_inboxes INDEXED BY idx_redis_usage_inboxes_processable_id
			WHERE status = 'pending' OR status = 'process_failed'
			ORDER BY id ASC
			LIMIT 1
		) AS oldest_popped_at`

// RedisUsageInboxBacklog is the processable inbox head and volume observed by
// /metrics. OldestPoppedAt is nil when no pending or retryable row exists.
type RedisUsageInboxBacklog struct {
	PendingCount   int64
	OldestPoppedAt *time.Time
}

func ReadRedisUsageInboxBacklog(ctx context.Context, db *gorm.DB) (RedisUsageInboxBacklog, error) {
	if ctx == nil {
		return RedisUsageInboxBacklog{}, fmt.Errorf("read redis usage inbox backlog: context is required")
	}
	if db == nil {
		return RedisUsageInboxBacklog{}, fmt.Errorf("read redis usage inbox backlog: database is required")
	}

	var backlog RedisUsageInboxBacklog
	if err := db.WithContext(ctx).Raw(redisUsageInboxBacklogQuery).Scan(&backlog).Error; err != nil {
		return RedisUsageInboxBacklog{}, fmt.Errorf("read redis usage inbox backlog: %w", err)
	}
	if backlog.OldestPoppedAt != nil {
		oldestUTC := backlog.OldestPoppedAt.UTC()
		backlog.OldestPoppedAt = &oldestUTC
	}
	return backlog, nil
}
