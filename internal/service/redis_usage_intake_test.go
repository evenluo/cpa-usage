package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
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

func TestRedisUsageIntakePersistsOnlyReplaySafePayload(t *testing.T) {
	db := openSyncTestDatabase(t)
	invalidMessage := `{invalid PRIVATE_INVALID_BODY`
	queue := &countingRedisQueue{messages: []string{
		`{"timestamp":"2026-08-31T08:00:00Z","provider":"claude","model":"sonnet","request_id":"safe-attempt","failed":true,"fail":{"status_code":429,"body":"PRIVATE_FAIL_BODY"},"response_headers":{"Set-Cookie":"PRIVATE_COOKIE"},"tokens":{"input_tokens":1,"output_tokens":2},"unknown":"PRIVATE_UNKNOWN"}`,
		invalidMessage,
	}}
	service := NewSyncServiceWithOptions(db, SyncServiceOptions{
		BaseURL:    "https://cpa.example.com",
		RedisQueue: queue,
	})

	pullResult, err := service.PullRedisUsageInbox(context.Background())
	if err != nil {
		t.Fatalf("pull replay-safe inbox: %v", err)
	}
	if pullResult == nil || pullResult.InsertedRows != 2 {
		t.Fatalf("unexpected pull result: %+v", pullResult)
	}
	var rows []entities.RedisUsageInbox
	if err := db.Order("id asc").Find(&rows).Error; err != nil {
		t.Fatalf("load replay-safe inbox rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two inbox rows, got %d", len(rows))
	}
	for _, forbidden := range []string{"PRIVATE_FAIL_BODY", "PRIVATE_COOKIE", "PRIVATE_UNKNOWN", "response_headers", `"body"`} {
		if strings.Contains(rows[0].RawMessage, forbidden) {
			t.Fatalf("replay-safe payload retained %q: %s", forbidden, rows[0].RawMessage)
		}
	}
	if !strings.Contains(rows[0].RawMessage, `"status_code":429`) || !strings.Contains(rows[0].RawMessage, `"request_id":"safe-attempt"`) {
		t.Fatalf("replay-safe payload lost required fields: %s", rows[0].RawMessage)
	}
	invalidDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(invalidMessage)))
	if strings.Contains(rows[1].RawMessage, "PRIVATE_INVALID_BODY") || !strings.Contains(rows[1].RawMessage, invalidDigest) || !strings.Contains(rows[1].RawMessage, fmt.Sprintf("bytes=%d", len(invalidMessage))) {
		t.Fatalf("invalid payload marker is not safe and observable: %s", rows[1].RawMessage)
	}

	processResult, err := service.ProcessRedisUsageInbox(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode redis usage message") {
		t.Fatalf("expected redacted invalid row to use decode_failed path, result=%+v err=%v", processResult, err)
	}
	if err := db.Order("id asc").Find(&rows).Error; err != nil {
		t.Fatalf("reload replay-safe inbox rows: %v", err)
	}
	if rows[0].Status != repository.RedisUsageInboxStatusProcessed || rows[1].Status != repository.RedisUsageInboxStatusDecodeFailed {
		t.Fatalf("unexpected replay-safe row lifecycle: %+v", rows)
	}
	var event entities.UsageEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatalf("load replay-safe event: %v", err)
	}
	if event.RequestID != "safe-attempt" || event.StatusCode != 429 {
		t.Fatalf("replay-safe projection lost event evidence: %+v", event)
	}
}
