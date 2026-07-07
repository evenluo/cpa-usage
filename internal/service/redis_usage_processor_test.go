package service

import (
	"testing"
	"time"

	"cpa-usage/internal/cpa"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
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

	result, err := newRedisUsageProcessor(db).process(fetchedAt)
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
	expectedKeys := []string{"processor-batch-1", "processor-batch-2"}
	for index, row := range stored {
		if row.ID != rows[index].ID || row.Status != repository.RedisUsageInboxStatusProcessed || row.UsageEventKey != expectedKeys[index] || row.ProcessedAt == nil || !row.ProcessedAt.Equal(fetchedAt) || !row.UpdatedAt.Equal(fetchedAt) {
			t.Fatalf("unexpected processed row %d: %+v", index, row)
		}
	}
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

	result, err := newRedisUsageProcessor(db).process(fetchedAt)
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

type errTemporaryProcessorFailure struct{}

func (errTemporaryProcessorFailure) Error() string {
	return "temporary insert failure"
}
