package service

import (
	"context"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const redisInboxProcessLimit = 1000

type redisUsageIntake struct {
	db       *gorm.DB
	queue    RedisQueue
	queueKey string
	now      func() time.Time
}

func newRedisUsageIntake(db *gorm.DB, queue RedisQueue, queueKey string, now func() time.Time) redisUsageIntake {
	return redisUsageIntake{db: db, queue: queue, queueKey: queueKey, now: now}
}

// pull 只 LPOP 队列消息并原样写入 redis_usage_inboxes。
func (i redisUsageIntake) pull(ctx context.Context) (*servicedto.RedisInboxPullResult, error) {
	if i.queue == nil {
		return nil, fmt.Errorf("sync service redis queue is nil")
	}

	fetchedAt := i.now().UTC()
	messages, err := i.queue.PopUsage(ctx)
	if err != nil {
		return &servicedto.RedisInboxPullResult{Status: "failed"}, fmt.Errorf("fetch redis usage: %w", err)
	}
	logrus.WithFields(logrus.Fields{
		"queue_key":     i.queueKey,
		"message_count": len(messages),
	}).Debug("redis usage batch popped")
	if len(messages) == 0 {
		return &servicedto.RedisInboxPullResult{Empty: true, Status: "empty"}, nil
	}

	inboxRows, err := insertRedisInboxMessages(i.db, i.queueKey, messages, fetchedAt)
	if err != nil {
		return &servicedto.RedisInboxPullResult{Status: "failed"}, fmt.Errorf("insert redis usage inbox: %w", err)
	}
	logrus.WithFields(logrus.Fields{
		"queue_key": i.queueKey,
		"row_count": len(inboxRows),
	}).Debug("redis usage inbox rows inserted")
	return &servicedto.RedisInboxPullResult{Status: "completed", InsertedRows: len(inboxRows)}, nil
}

// process 只读取 pending/process_failed inbox 行并写入 usage_events。
func (i redisUsageIntake) process(ctx context.Context) (*servicedto.RedisBatchSyncResult, error) {
	fetchedAt := i.now().UTC()
	processableRows, err := repository.ListProcessableRedisUsageInbox(i.db, redisInboxProcessLimit)
	if err != nil {
		return &servicedto.RedisBatchSyncResult{Status: "failed"}, fmt.Errorf("list processable redis usage inbox: %w", err)
	}
	if len(processableRows) == 0 {
		return &servicedto.RedisBatchSyncResult{Empty: true, Status: "empty"}, nil
	}
	logrus.WithField("row_count", len(processableRows)).Debug("redis usage inbox rows found for processing")
	return i.processRows(processableRows, fetchedAt)
}

// processRows 从已落库原始消息解码并写入事件；坏消息标记为 decode_failed，不阻塞同批其它数据。
func (i redisUsageIntake) processRows(inboxRows []entities.RedisUsageInbox, fetchedAt time.Time) (*servicedto.RedisBatchSyncResult, error) {
	logrus.WithField("row_count", len(inboxRows)).Debug("redis usage inbox processing started")
	validRows := make([]entities.RedisUsageInbox, 0, len(inboxRows))
	events := make([]entities.UsageEvent, 0, len(inboxRows))
	decodeErrs := make([]error, 0)
	for _, row := range inboxRows {
		event, _, decodeErr := DecodeRedisUsageMessage(row.RawMessage, fetchedAt)
		if decodeErr != nil {
			if markErr := repository.MarkRedisUsageInboxDecodeFailed(i.db, row.ID, decodeErr); markErr != nil {
				return &servicedto.RedisBatchSyncResult{Status: "failed"}, fmt.Errorf("mark redis usage inbox decode failed: %w", markErr)
			}
			decodeErrs = append(decodeErrs, decodeErr)
			continue
		}
		validRows = append(validRows, row)
		events = append(events, event)
	}
	decodeErr := joinErrors(decodeErrs...)
	logrus.WithFields(logrus.Fields{
		"row_count":           len(inboxRows),
		"valid_event_count":   len(events),
		"decode_failed_count": len(decodeErrs),
	}).Debug("redis usage inbox rows decoded")
	if len(events) == 0 {
		if decodeErr != nil {
			return &servicedto.RedisBatchSyncResult{Status: "completed_with_warnings"}, decodeErr
		}
		return &servicedto.RedisBatchSyncResult{Empty: true, Status: "empty"}, nil
	}

	logrus.WithField("event_count", len(events)).Debug("redis usage events persistence started")
	result, err := i.persistEvents(events)
	if result == nil {
		markRedisInboxRowsProcessFailed(i.db, validRows, err)
		return nil, err
	}
	if err != nil && result.Status == "failed" {
		markRedisInboxRowsProcessFailed(i.db, validRows, err)
		return &servicedto.RedisBatchSyncResult{Status: result.Status}, err
	}
	for index, row := range validRows {
		if markErr := repository.MarkRedisUsageInboxProcessed(i.db, row.ID, events[index].EventKey, fetchedAt); markErr != nil {
			return &servicedto.RedisBatchSyncResult{Status: "failed"}, fmt.Errorf("mark redis usage inbox processed: %w", markErr)
		}
	}
	logrus.WithFields(logrus.Fields{
		"processed_rows":  len(validRows),
		"inserted_events": result.InsertedEvents,
		"deduped_events":  result.DedupedEvents,
		"status":          result.Status,
	}).Debug("redis usage inbox rows processed")

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

func (i redisUsageIntake) persistEvents(events []entities.UsageEvent) (*servicedto.SyncResult, error) {
	var err error
	events, err = alignUsageEventKeysWithExistingCanonicalEvents(i.db, events)
	if err != nil {
		return &servicedto.SyncResult{Status: "failed"}, fmt.Errorf("align usage events: %w", err)
	}
	logrus.WithField("event_count", len(events)).Debug("usage events insert started")
	inserted, deduped, err := repository.InsertUsageEvents(i.db, events)
	if err != nil {
		return &servicedto.SyncResult{Status: "failed"}, fmt.Errorf("insert usage events: %w", err)
	}
	logrus.WithFields(logrus.Fields{
		"inserted_events": inserted,
		"deduped_events":  deduped,
	}).Debug("usage events insert finished")
	return &servicedto.SyncResult{Status: "completed", InsertedEvents: inserted, DedupedEvents: deduped}, nil
}

func insertRedisInboxMessages(db *gorm.DB, queueKey string, messages []string, poppedAt time.Time) ([]entities.RedisUsageInbox, error) {
	inputs := make([]repodto.RedisInboxInsert, 0, len(messages))
	for _, message := range messages {
		inputs = append(inputs, repodto.RedisInboxInsert{
			QueueKey:   queueKey,
			RawMessage: message,
			PoppedAt:   poppedAt,
		})
	}
	return repository.InsertRedisUsageInboxMessages(db, inputs)
}

func markRedisInboxRowsProcessFailed(db *gorm.DB, rows []entities.RedisUsageInbox, err error) {
	if err == nil {
		return
	}
	for _, row := range rows {
		if markErr := repository.MarkRedisUsageInboxProcessFailed(db, row.ID, err); markErr != nil {
			logrus.WithError(markErr).WithField("inbox_id", row.ID).Warn("failed to mark redis usage inbox process failure")
			continue
		}
		var stored entities.RedisUsageInbox
		if loadErr := db.First(&stored, row.ID).Error; loadErr != nil {
			logrus.WithError(loadErr).WithField("inbox_id", row.ID).Warn("failed to load redis usage inbox after process failure")
			continue
		}
		if stored.Status == repository.RedisUsageInboxStatusDiscarded {
			logrus.WithFields(logrus.Fields{
				"inbox_id":      stored.ID,
				"queue_key":     stored.QueueKey,
				"message_hash":  stored.MessageHash,
				"attempt_count": stored.AttemptCount,
				"last_error":    stored.LastError,
				"popped_at":     stored.PoppedAt,
			}).Warn("discarded redis usage inbox row after repeated process failures")
		}
	}
}
