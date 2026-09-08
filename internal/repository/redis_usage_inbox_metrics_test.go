package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/repository/dto"
)

func TestReadRedisUsageInboxBacklogReturnsCountAndHeadAgeSource(t *testing.T) {
	db := openTestDatabase(t)
	poppedAt := time.Date(2026, 9, 8, 4, 0, 0, 0, time.UTC)
	rows, err := InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{
		{QueueKey: "queue", RawMessage: `{"request_id":"processed"}`, PoppedAt: poppedAt},
		{QueueKey: "queue", RawMessage: `{"request_id":"retry"}`, PoppedAt: poppedAt.Add(time.Second)},
		{QueueKey: "queue", RawMessage: `{"request_id":"pending"}`, PoppedAt: poppedAt.Add(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("insert inbox messages: %v", err)
	}
	if err := MarkRedisUsageInboxProcessed(db, rows[0].ID, "event-1", poppedAt.Add(time.Minute)); err != nil {
		t.Fatalf("mark processed: %v", err)
	}
	if err := MarkRedisUsageInboxProcessFailed(db, rows[1].ID, fmt.Errorf("retry")); err != nil {
		t.Fatalf("mark retryable: %v", err)
	}

	backlog, err := ReadRedisUsageInboxBacklog(context.Background(), db)
	if err != nil {
		t.Fatalf("ReadRedisUsageInboxBacklog returned error: %v", err)
	}
	if backlog.PendingCount != 2 {
		t.Fatalf("expected two processable rows, got %d", backlog.PendingCount)
	}
	if backlog.OldestPoppedAt == nil || !backlog.OldestPoppedAt.Equal(poppedAt.Add(time.Second)) {
		t.Fatalf("expected retry row at the processable head, got %v", backlog.OldestPoppedAt)
	}
}

func TestReadRedisUsageInboxBacklogUsesProcessableIndexForBothSubqueries(t *testing.T) {
	db := openTestDatabase(t)
	type queryPlanRow struct {
		Detail string
	}
	var plan []queryPlanRow
	if err := db.Raw("EXPLAIN QUERY PLAN " + redisUsageInboxBacklogQuery).Scan(&plan).Error; err != nil {
		t.Fatalf("explain backlog query: %v", err)
	}
	var details []string
	for _, row := range plan {
		details = append(details, row.Detail)
	}
	if uses := strings.Count(strings.Join(details, "\n"), "idx_redis_usage_inboxes_processable_id"); uses < 2 {
		t.Fatalf("expected both scalar subqueries to use processable index, plan:\n%s", strings.Join(details, "\n"))
	}
}

func TestReadRedisUsageInboxBacklogHonorsCanceledContext(t *testing.T) {
	db := openTestDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ReadRedisUsageInboxBacklog(ctx, db)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
