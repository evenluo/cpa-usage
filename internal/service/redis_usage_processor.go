package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	servicedto "cpa-usage/internal/service/dto"
	"gorm.io/gorm"
)

type redisUsageProcessor struct {
	db               *gorm.DB
	eventKeyAssigner CanonicalEventKeyAssigner
}

func newRedisUsageProcessor(db *gorm.DB) redisUsageProcessor {
	return redisUsageProcessor{
		db:               db,
		eventKeyAssigner: NewCanonicalEventKeyAssigner(NewRepositoryCanonicalEventLookup(db)),
	}
}

func (p redisUsageProcessor) process(ctx context.Context, now time.Time) (*servicedto.RedisBatchSyncResult, error) {
	processableRows, err := repository.ListProcessableRedisUsageInbox(p.db, redisInboxProcessLimit)
	if err != nil {
		return &servicedto.RedisBatchSyncResult{Status: "failed"}, fmt.Errorf("list processable redis usage inbox: %w", err)
	}
	if len(processableRows) == 0 {
		return &servicedto.RedisBatchSyncResult{Empty: true, Status: "empty"}, nil
	}
	slog.Debug("redis usage inbox rows found for processing", "row_count", len(processableRows))
	return p.processRows(ctx, processableRows, now.UTC())
}

// processRows 从已落库原始消息解码并写入事件；坏消息标记为 decode_failed，不阻塞同批其它数据。
func (p redisUsageProcessor) processRows(ctx context.Context, inboxRows []entities.RedisUsageInbox, fetchedAt time.Time) (*servicedto.RedisBatchSyncResult, error) {
	slog.Debug("redis usage inbox processing started", "row_count", len(inboxRows))
	validRows := make([]entities.RedisUsageInbox, 0, len(inboxRows))
	events := make([]entities.UsageEvent, 0, len(inboxRows))
	decodeErrs := make([]error, 0)
	for _, row := range inboxRows {
		event, _, decodeErr := DecodeRedisUsageMessage(row.RawMessage, fetchedAt)
		if decodeErr != nil {
			if markErr := repository.MarkRedisUsageInboxDecodeFailed(p.db, row.ID, decodeErr); markErr != nil {
				return &servicedto.RedisBatchSyncResult{Status: "failed"}, fmt.Errorf("mark redis usage inbox decode failed: %w", markErr)
			}
			decodeErrs = append(decodeErrs, decodeErr)
			continue
		}
		validRows = append(validRows, row)
		events = append(events, event)
	}
	decodeErr := joinErrors(decodeErrs...)
	slog.Debug("redis usage inbox rows decoded",
		"row_count", len(inboxRows),
		"valid_event_count", len(events),
		"decode_failed_count", len(decodeErrs),
	)
	if len(events) == 0 {
		if decodeErr != nil {
			return &servicedto.RedisBatchSyncResult{Status: "completed_with_warnings"}, decodeErr
		}
		return &servicedto.RedisBatchSyncResult{Empty: true, Status: "empty"}, nil
	}

	slog.Debug("redis usage events persistence started", "event_count", len(events))
	result, err := p.persistEvents(ctx, events)
	if result == nil {
		markRedisInboxRowsProcessFailed(p.db, validRows, err)
		return nil, err
	}
	if err != nil && result.Status == "failed" {
		markRedisInboxRowsProcessFailed(p.db, validRows, err)
		return &servicedto.RedisBatchSyncResult{Status: result.Status}, err
	}
	marks := make([]repository.RedisUsageInboxProcessedMark, 0, len(validRows))
	for index, row := range validRows {
		marks = append(marks, repository.RedisUsageInboxProcessedMark{
			ID:            row.ID,
			UsageEventKey: events[index].EventKey,
		})
	}
	if markErr := repository.MarkRedisUsageInboxProcessedBatch(p.db, marks, fetchedAt); markErr != nil {
		return &servicedto.RedisBatchSyncResult{Status: "failed"}, fmt.Errorf("mark redis usage inbox processed: %w", markErr)
	}
	slog.Debug("redis usage inbox rows processed",
		"processed_rows", len(validRows),
		"inserted_events", result.InsertedEvents,
		"deduped_events", result.DedupedEvents,
		"status", result.Status,
	)

	status := result.Status
	returnErr := err
	if decodeErr != nil {
		status = "completed_with_warnings"
		if returnErr != nil {
			returnErr = joinErrors(returnErr, decodeErr)
		} else {
			returnErr = decodeErr
		}
	}
	return &servicedto.RedisBatchSyncResult{
		Status:         status,
		InsertedEvents: result.InsertedEvents,
		DedupedEvents:  result.DedupedEvents,
	}, returnErr
}

func (p redisUsageProcessor) persistEvents(ctx context.Context, events []entities.UsageEvent) (*servicedto.SyncResult, error) {
	if err := p.eventKeyAssigner.Assign(ctx, events); err != nil {
		return &servicedto.SyncResult{Status: "failed"}, fmt.Errorf("assign canonical event keys: %w", err)
	}
	slog.Debug("usage events insert started", "event_count", len(events))
	inserted, deduped, err := repository.InsertUsageEvents(p.db, events)
	if err != nil {
		return &servicedto.SyncResult{Status: "failed"}, fmt.Errorf("insert usage events: %w", err)
	}
	slog.Debug("usage events insert finished", "inserted_events", inserted, "deduped_events", deduped)
	return &servicedto.SyncResult{Status: "completed", InsertedEvents: inserted, DedupedEvents: deduped}, nil
}

func markRedisInboxRowsProcessFailed(db *gorm.DB, rows []entities.RedisUsageInbox, err error) {
	if err == nil {
		return
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if markErr := repository.MarkRedisUsageInboxProcessFailedBatch(db, ids, err); markErr != nil {
		slog.Warn("failed to mark redis usage inbox process failures", "error", markErr)
		return
	}
	for _, row := range rows {
		var stored entities.RedisUsageInbox
		if loadErr := db.First(&stored, row.ID).Error; loadErr != nil {
			slog.Warn("failed to load redis usage inbox after process failure", "error", loadErr, "inbox_id", row.ID)
			continue
		}
		if stored.Status == repository.RedisUsageInboxStatusDiscarded {
			slog.Warn("discarded redis usage inbox row after repeated process failures",
				"inbox_id", stored.ID,
				"queue_key", stored.QueueKey,
				"message_hash", stored.MessageHash,
				"attempt_count", stored.AttemptCount,
				"last_error", stored.LastError,
				"popped_at", stored.PoppedAt,
			)
		}
	}
}
