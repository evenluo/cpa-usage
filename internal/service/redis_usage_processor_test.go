package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/cpa"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func TestRedisUsageProcessorBatchMarksProcessedRows(t *testing.T) {
	db := openSyncTestDatabase(t)
	fetchedAt := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{
		{
			QueueKey:   cpa.ManagementUsageQueueKey,
			RawMessage: `{"timestamp":"2026-04-27T08:00:00Z","provider":"claude","model":"sonnet","request_id":"processor-batch-1","tokens":{"input_tokens":1,"output_tokens":2}}`,
			PoppedAt:   fetchedAt,
		},
		{
			QueueKey:   cpa.ManagementUsageQueueKey,
			RawMessage: `{"timestamp":"2026-04-27T08:01:00Z","provider":"claude","model":"sonnet","request_id":"processor-batch-2","tokens":{"input_tokens":3,"output_tokens":4}}`,
			PoppedAt:   fetchedAt,
		},
	})
	if err != nil {
		t.Fatalf("seed inbox rows: %v", err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), fetchedAt)
	if err != nil {
		t.Fatalf("processor returned error: %v", err)
	}
	if result == nil || result.Status != "completed" || result.InsertedEvents != 2 {
		t.Fatalf("unexpected processor result: %+v", result)
	}

	var stored []entities.RedisUsageInbox
	if err := db.Order("id asc").Find(&stored).Error; err != nil {
		t.Fatalf("load inbox rows: %v", err)
	}
	expectedKeys := []string{redisUsageAttemptEventKey(rows[0].ID), redisUsageAttemptEventKey(rows[1].ID)}
	for index, row := range stored {
		if row.ID != rows[index].ID || row.Status != repository.RedisUsageInboxStatusProcessed || row.UsageEventKey != expectedKeys[index] || row.ProcessedAt == nil || !row.ProcessedAt.Equal(fetchedAt) || !row.UpdatedAt.Equal(fetchedAt) {
			t.Fatalf("unexpected processed row %d: %+v", index, row)
		}
	}
}

func TestRedisUsageProcessorPreservesAttemptsAndDedupesOnlyInboxReplay(t *testing.T) {
	db := openSyncTestDatabase(t)
	poppedAt := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	requestID := "shared-request"
	failedAttempt := `{"timestamp":"2026-08-31T08:00:00Z","provider":"claude","executor_type":"claude","model":"sonnet","request_id":"shared-request","reasoning_effort":"high","service_tier":"priority","failed":true,"fail":{"status_code":429,"body":"do not persist"},"tokens":{"input_tokens":1,"cached_tokens":5,"cache_read_tokens":3,"cache_creation_tokens":2}}`
	successAttempt := `{"timestamp":"2026-08-31T08:00:01Z","provider":"openai","model":"gpt-5","request_id":"shared-request","failed":false,"fail":{"status_code":200},"tokens":{"input_tokens":2,"output_tokens":3}}`
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{
		{QueueKey: cpa.ManagementUsageQueueKey, RawMessage: failedAttempt, PoppedAt: poppedAt},
		{QueueKey: cpa.ManagementUsageQueueKey, RawMessage: successAttempt, PoppedAt: poppedAt.Add(time.Second)},
		// Byte-identical queue records still represent two destructive pops and therefore two attempts.
		{QueueKey: cpa.ManagementUsageQueueKey, RawMessage: successAttempt, PoppedAt: poppedAt.Add(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("seed inbox attempts: %v", err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), poppedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("process attempts: %v", err)
	}
	if result.InsertedEvents != 3 || result.DedupedEvents != 0 {
		t.Fatalf("expected every popped record to persist as an attempt, got %+v", result)
	}
	var attempts []entities.UsageEvent
	if err := db.Where("request_id = ?", requestID).Order("id asc").Find(&attempts).Error; err != nil {
		t.Fatalf("load request attempts: %v", err)
	}
	if len(attempts) != 3 {
		t.Fatalf("expected three attempts grouped by request_id, got %+v", attempts)
	}
	for index, attempt := range attempts {
		if attempt.EventKey != redisUsageAttemptEventKey(rows[index].ID) || attempt.RequestID != requestID {
			t.Fatalf("attempt %d lost inbox identity or request grouping: %+v", index, attempt)
		}
	}
	if attempts[0].StatusCode != 429 || attempts[0].ExecutorType != "claude" || attempts[0].ReasoningEffort != "high" || attempts[0].ServiceTier != "priority" {
		t.Fatalf("expected safe attempt metadata to persist, got %+v", attempts[0])
	}
	if attempts[0].CachedTokens != 5 || attempts[0].CacheReadTokens == nil || *attempts[0].CacheReadTokens != 3 || attempts[0].CacheCreationTokens == nil || *attempts[0].CacheCreationTokens != 2 {
		t.Fatalf("expected exact cache fields and generic compatibility projection to persist, got %+v", attempts[0])
	}

	// Reprocessing the same persisted inbox row is a replay, not a fourth attempt.
	if err := db.Model(&entities.RedisUsageInbox{}).Where("id = ?", rows[0].ID).Updates(map[string]any{
		"status":          repository.RedisUsageInboxStatusPending,
		"usage_event_key": "",
		"processed_at":    nil,
	}).Error; err != nil {
		t.Fatalf("reset inbox row for replay: %v", err)
	}
	result, err = newRedisUsageProcessor(db).process(context.Background(), poppedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("replay inbox attempt: %v", err)
	}
	if result.InsertedEvents != 0 || result.DedupedEvents != 1 {
		t.Fatalf("expected same inbox replay to dedupe, got %+v", result)
	}
	assertUsageEventCount(t, db, 3)
}

func TestRedisUsageProcessorUpgradeRetryKeepsLegacyRequestIDIdentity(t *testing.T) {
	db := openSyncTestDatabase(t)
	poppedAt := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	rawMessage := `{"timestamp":"2026-08-31T08:00:00Z","provider":"claude","model":"sonnet","request_id":"legacy-partial-write","tokens":{"input_tokens":1,"output_tokens":2}}`
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{{
		QueueKey:   legacyManagementUsageQueueKey,
		RawMessage: rawMessage,
		PoppedAt:   poppedAt,
	}})
	if err != nil {
		t.Fatalf("seed legacy inbox row: %v", err)
	}
	legacyEvent, _, err := DecodeRedisUsageMessage(rawMessage, poppedAt)
	if err != nil {
		t.Fatalf("decode legacy event: %v", err)
	}
	inserted, deduped, err := repository.InsertUsageEvents(db, []entities.UsageEvent{legacyEvent})
	if err != nil || inserted != 1 || deduped != 0 {
		t.Fatalf("seed legacy partial event write: inserted=%d deduped=%d err=%v", inserted, deduped, err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), poppedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("retry legacy inbox row after upgrade: %v", err)
	}
	if result.InsertedEvents != 0 || result.DedupedEvents != 1 {
		t.Fatalf("expected legacy partial write to dedupe, got %+v", result)
	}
	assertUsageEventCount(t, db, 1)
	assertProcessedRedisInboxRow(t, db, rows[0].ID, legacyEvent.EventKey, poppedAt.Add(time.Minute))
}

func TestRedisUsageProcessorUpgradeRetryKeepsLegacyBuiltIdentity(t *testing.T) {
	db := openSyncTestDatabase(t)
	poppedAt := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	rawMessage := `{"provider":"claude","model":"sonnet","tokens":{"input_tokens":1,"output_tokens":2}}`
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{{
		QueueKey:   legacyManagementUsageQueueKey,
		RawMessage: rawMessage,
		PoppedAt:   poppedAt,
	}})
	if err != nil {
		t.Fatalf("seed legacy inbox row: %v", err)
	}
	legacyEvent, _, err := DecodeRedisUsageMessage(rawMessage, poppedAt)
	if err != nil {
		t.Fatalf("decode legacy event: %v", err)
	}
	inserted, deduped, err := repository.InsertUsageEvents(db, []entities.UsageEvent{legacyEvent})
	if err != nil || inserted != 1 || deduped != 0 {
		t.Fatalf("seed legacy partial event write: inserted=%d deduped=%d err=%v", inserted, deduped, err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), poppedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("retry legacy inbox row after upgrade: %v", err)
	}
	if result.InsertedEvents != 0 || result.DedupedEvents != 1 {
		t.Fatalf("expected legacy built identity to dedupe, got %+v", result)
	}
	assertUsageEventCount(t, db, 1)
	assertProcessedRedisInboxRow(t, db, rows[0].ID, legacyEvent.EventKey, poppedAt.Add(time.Minute))
}

func TestRedisUsageProcessorRetriesOnlyProcessableRows(t *testing.T) {
	db := openSyncTestDatabase(t)
	fetchedAt := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{
		{
			QueueKey:   cpa.ManagementUsageQueueKey,
			RawMessage: `{"timestamp":"2026-04-27T08:00:00Z","provider":"claude","model":"sonnet","request_id":"processor-pending","tokens":{"input_tokens":1,"output_tokens":2}}`,
			PoppedAt:   fetchedAt,
		},
		{
			QueueKey:   cpa.ManagementUsageQueueKey,
			RawMessage: `{"timestamp":"2026-04-27T08:01:00Z","provider":"claude","model":"sonnet","request_id":"processor-retry","tokens":{"input_tokens":3,"output_tokens":4}}`,
			PoppedAt:   fetchedAt,
		},
		{
			QueueKey:   cpa.ManagementUsageQueueKey,
			RawMessage: `{"timestamp":"2026-04-27T08:02:00Z","provider":"claude","model":"sonnet","request_id":"processor-discarded","tokens":{"input_tokens":5,"output_tokens":6}}`,
			PoppedAt:   fetchedAt,
		},
	})
	if err != nil {
		t.Fatalf("seed inbox rows: %v", err)
	}
	if err := repository.MarkRedisUsageInboxProcessFailed(db, rows[1].ID, errTemporaryProcessorFailure{}); err != nil {
		t.Fatalf("mark process failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := repository.MarkRedisUsageInboxProcessFailed(db, rows[2].ID, errTemporaryProcessorFailure{}); err != nil {
			t.Fatalf("mark discarded: %v", err)
		}
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), fetchedAt)
	if err != nil {
		t.Fatalf("processor returned error: %v", err)
	}
	if result == nil || result.Status != "completed" || result.InsertedEvents != 2 {
		t.Fatalf("expected pending and process_failed rows only, got %+v", result)
	}

	var stored []entities.RedisUsageInbox
	if err := db.Order("id asc").Find(&stored).Error; err != nil {
		t.Fatalf("load inbox rows: %v", err)
	}
	if stored[0].Status != repository.RedisUsageInboxStatusProcessed || stored[1].Status != repository.RedisUsageInboxStatusProcessed || stored[2].Status != repository.RedisUsageInboxStatusDiscarded {
		t.Fatalf("unexpected retry/discard statuses: %+v", stored)
	}
}

func TestRedisUsageProcessorRetryAfterEventWriteFailureUsesPersistedPoppedAt(t *testing.T) {
	db := openSyncTestDatabase(t)
	row, poppedAt, expectedKey := seedFallbackRedisInboxRow(t, db)
	firstProcessedAt := poppedAt.Add(time.Minute)
	retryProcessedAt := poppedAt.Add(24 * time.Hour)
	if err := db.Exec(`CREATE TRIGGER fail_usage_event_insert BEFORE INSERT ON usage_events BEGIN SELECT RAISE(FAIL, 'forced event insert failure'); END`).Error; err != nil {
		t.Fatalf("create event insert failure trigger: %v", err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), firstProcessedAt)
	if err == nil || result == nil || result.Status != "failed" || !strings.Contains(err.Error(), "insert usage events") {
		t.Fatalf("expected event write failure, got result=%+v err=%v", result, err)
	}
	var failedRow entities.RedisUsageInbox
	if err := db.First(&failedRow, row.ID).Error; err != nil {
		t.Fatalf("load failed inbox row: %v", err)
	}
	if failedRow.Status != repository.RedisUsageInboxStatusProcessFailed || failedRow.AttemptCount != 1 {
		t.Fatalf("expected retryable process failure, got %+v", failedRow)
	}
	assertUsageEventCount(t, db, 0)

	if err := db.Exec(`DROP TRIGGER fail_usage_event_insert`).Error; err != nil {
		t.Fatalf("drop event insert failure trigger: %v", err)
	}
	result, err = newRedisUsageProcessor(db).process(context.Background(), retryProcessedAt)
	if err != nil {
		t.Fatalf("retry processor returned error: %v", err)
	}
	if result == nil || result.Status != "completed" || result.InsertedEvents != 1 || result.DedupedEvents != 0 {
		t.Fatalf("unexpected retry result: %+v", result)
	}

	assertFallbackRedisEvent(t, db, expectedKey, poppedAt)
	assertProcessedRedisInboxRow(t, db, row.ID, expectedKey, retryProcessedAt)
}

func TestRedisUsageProcessorRetryAfterProcessedMarkFailureKeepsEventIdentity(t *testing.T) {
	db := openSyncTestDatabase(t)
	row, poppedAt, expectedKey := seedFallbackRedisInboxRow(t, db)
	firstProcessedAt := poppedAt.Add(time.Minute)
	retryProcessedAt := poppedAt.Add(24 * time.Hour)
	if err := db.Exec(`CREATE TRIGGER fail_inbox_processed_mark BEFORE UPDATE OF status ON redis_usage_inboxes WHEN NEW.status = 'processed' BEGIN SELECT RAISE(FAIL, 'forced processed mark failure'); END`).Error; err != nil {
		t.Fatalf("create processed mark failure trigger: %v", err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), firstProcessedAt)
	if err == nil || result == nil || result.Status != "failed" || !strings.Contains(err.Error(), "mark redis usage inbox processed") {
		t.Fatalf("expected processed mark failure, got result=%+v err=%v", result, err)
	}
	assertFallbackRedisEvent(t, db, expectedKey, poppedAt)
	var pendingRow entities.RedisUsageInbox
	if err := db.First(&pendingRow, row.ID).Error; err != nil {
		t.Fatalf("load pending inbox row: %v", err)
	}
	if pendingRow.Status != repository.RedisUsageInboxStatusPending || pendingRow.UsageEventKey != "" {
		t.Fatalf("expected row to remain pending after mark failure, got %+v", pendingRow)
	}

	if err := db.Exec(`DROP TRIGGER fail_inbox_processed_mark`).Error; err != nil {
		t.Fatalf("drop processed mark failure trigger: %v", err)
	}
	result, err = newRedisUsageProcessor(db).process(context.Background(), retryProcessedAt)
	if err != nil {
		t.Fatalf("retry processor returned error: %v", err)
	}
	if result == nil || result.Status != "completed" || result.InsertedEvents != 0 || result.DedupedEvents != 1 {
		t.Fatalf("expected retry to dedupe the same event, got %+v", result)
	}
	assertUsageEventCount(t, db, 1)
	assertFallbackRedisEvent(t, db, expectedKey, poppedAt)
	assertProcessedRedisInboxRow(t, db, row.ID, expectedKey, retryProcessedAt)
}

func TestRedisUsageProcessorRejectsZeroPersistedPoppedAt(t *testing.T) {
	db := openSyncTestDatabase(t)
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{{
		QueueKey:   cpa.ManagementUsageQueueKey,
		RawMessage: `{"provider":"claude","model":"sonnet"}`,
		PoppedAt:   time.Time{},
	}})
	if err != nil {
		t.Fatalf("seed inbox row: %v", err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC))
	if err == nil || result == nil || result.Status != "completed_with_warnings" || !strings.Contains(err.Error(), "persisted invariant failed: popped_at is zero") {
		t.Fatalf("expected explicit popped_at invariant failure, got result=%+v err=%v", result, err)
	}
	var stored entities.RedisUsageInbox
	if err := db.First(&stored, rows[0].ID).Error; err != nil {
		t.Fatalf("load inbox row: %v", err)
	}
	if stored.Status != repository.RedisUsageInboxStatusDecodeFailed || !strings.Contains(stored.LastError, "popped_at is zero") {
		t.Fatalf("expected zero popped_at row to be isolated, got %+v", stored)
	}
	assertUsageEventCount(t, db, 0)
}

func assertFallbackRedisEvent(t *testing.T, db *gorm.DB, expectedKey string, expectedTimestamp time.Time) {
	t.Helper()
	var event entities.UsageEvent
	if err := db.Where("event_key = ?", expectedKey).First(&event).Error; err != nil {
		t.Fatalf("load fallback usage event: %v", err)
	}
	if !event.Timestamp.Equal(expectedTimestamp) {
		t.Fatalf("expected fallback timestamp %s, got %s", expectedTimestamp, event.Timestamp)
	}
}

func seedFallbackRedisInboxRow(t *testing.T, db *gorm.DB) (entities.RedisUsageInbox, time.Time, string) {
	t.Helper()
	poppedAt := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{{
		QueueKey:   cpa.ManagementUsageQueueKey,
		RawMessage: `{"provider":"claude","model":"sonnet","tokens":{"input_tokens":1,"output_tokens":2}}`,
		PoppedAt:   poppedAt,
	}})
	if err != nil {
		t.Fatalf("seed inbox row: %v", err)
	}
	expectedKey := redisUsageAttemptEventKey(rows[0].ID)
	return rows[0], poppedAt, expectedKey
}

func assertProcessedRedisInboxRow(t *testing.T, db *gorm.DB, rowID uint, expectedKey string, expectedProcessedAt time.Time) {
	t.Helper()
	var row entities.RedisUsageInbox
	if err := db.First(&row, rowID).Error; err != nil {
		t.Fatalf("load processed inbox row: %v", err)
	}
	if row.Status != repository.RedisUsageInboxStatusProcessed || row.UsageEventKey != expectedKey || row.ProcessedAt == nil || !row.ProcessedAt.Equal(expectedProcessedAt) {
		t.Fatalf("expected deterministic retry with processing time kept separate, got %+v", row)
	}
}

type errTemporaryProcessorFailure struct{}

func (errTemporaryProcessorFailure) Error() string {
	return "temporary insert failure"
}
