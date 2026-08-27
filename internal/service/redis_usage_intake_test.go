package service

import (
	"context"
	"strings"
	"testing"

	"cpa-usage/internal/entities"
)

type countingRedisQueue struct {
	calls    int
	messages []string
}

func (q *countingRedisQueue) PopUsage(context.Context) ([]string, error) {
	q.calls++
	return q.messages, nil
}

func TestRedisUsageIntakeReportsLossWithoutRetryWhenInboxWriteFails(t *testing.T) {
	db := openSyncTestDatabase(t)
	if err := db.Exec(`CREATE TRIGGER fail_redis_inbox_insert BEFORE INSERT ON redis_usage_inboxes BEGIN SELECT RAISE(FAIL, 'forced inbox write failure'); END`).Error; err != nil {
		t.Fatalf("create inbox insert failure trigger: %v", err)
	}
	logs := captureSyncDebugLogs(t)
	queue := &countingRedisQueue{messages: []string{`{"request_id":"lost-after-pop"}`}}
	service := NewSyncServiceWithOptions(db, SyncServiceOptions{
		BaseURL:    "https://cpa.example.com",
		RedisQueue: queue,
	})

	result, err := service.PullRedisUsageInbox(context.Background())
	if err == nil || result == nil || result.Status != "failed" || !strings.Contains(err.Error(), "messages lost after destructive queue pop") {
		t.Fatalf("expected observable inbox persistence loss, got result=%+v err=%v", result, err)
	}
	if queue.calls != 1 {
		t.Fatalf("expected exactly one destructive queue call and no Adapter retry, got %d", queue.calls)
	}
	var inboxCount int64
	if err := db.Model(&entities.RedisUsageInbox{}).Count(&inboxCount).Error; err != nil {
		t.Fatalf("count inbox rows: %v", err)
	}
	if inboxCount != 0 {
		t.Fatalf("expected failed inbox transaction to store no rows, got %d", inboxCount)
	}
	if output := logs.String(); !strings.Contains(output, "redis usage messages lost after destructive queue pop") || strings.Contains(output, "lost-after-pop") {
		t.Fatalf("expected payload-safe observable loss log, got:\n%s", output)
	}
}

var _ RedisQueue = (*countingRedisQueue)(nil)
