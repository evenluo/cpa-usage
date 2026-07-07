package service

import (
	"fmt"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type redisUsageProcessor struct {
	db *gorm.DB
}

type canonicalUsageEventLookup struct {
	EventKey   string
	Referenced bool
}

func newRedisUsageProcessor(db *gorm.DB) redisUsageProcessor {
	return redisUsageProcessor{db: db}
}

func (p redisUsageProcessor) process(now time.Time) (*servicedto.RedisBatchSyncResult, error) {
	processableRows, err := repository.ListProcessableRedisUsageInbox(p.db, redisInboxProcessLimit)
	if err != nil {
		return &servicedto.RedisBatchSyncResult{Status: "failed"}, fmt.Errorf("list processable redis usage inbox: %w", err)
	}
	if len(processableRows) == 0 {
		return &servicedto.RedisBatchSyncResult{Empty: true, Status: "empty"}, nil
	}
	logrus.WithField("row_count", len(processableRows)).Debug("redis usage inbox rows found for processing")
	return p.processRows(processableRows, now.UTC())
}

// processRows 从已落库原始消息解码并写入事件；坏消息标记为 decode_failed，不阻塞同批其它数据。
func (p redisUsageProcessor) processRows(inboxRows []entities.RedisUsageInbox, fetchedAt time.Time) (*servicedto.RedisBatchSyncResult, error) {
	logrus.WithField("row_count", len(inboxRows)).Debug("redis usage inbox processing started")
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
	result, err := p.persistEvents(events)
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

func (p redisUsageProcessor) persistEvents(events []entities.UsageEvent) (*servicedto.SyncResult, error) {
	var err error
	events, err = alignUsageEventKeysWithExistingCanonicalEvents(p.db, events)
	if err != nil {
		return &servicedto.SyncResult{Status: "failed"}, fmt.Errorf("align usage events: %w", err)
	}
	logrus.WithField("event_count", len(events)).Debug("usage events insert started")
	inserted, deduped, err := repository.InsertUsageEvents(p.db, events)
	if err != nil {
		return &servicedto.SyncResult{Status: "failed"}, fmt.Errorf("insert usage events: %w", err)
	}
	logrus.WithFields(logrus.Fields{
		"inserted_events": inserted,
		"deduped_events":  deduped,
	}).Debug("usage events insert finished")
	return &servicedto.SyncResult{Status: "completed", InsertedEvents: inserted, DedupedEvents: deduped}, nil
}

func alignUsageEventKeysWithExistingCanonicalEvents(db *gorm.DB, events []entities.UsageEvent) ([]entities.UsageEvent, error) {
	if len(events) == 0 {
		return events, nil
	}
	for i := range events {
		events[i].Timestamp = events[i].Timestamp.UTC()
	}
	lookup, err := loadCanonicalUsageEventLookup(db, events)
	if err != nil {
		return nil, err
	}
	canonicalEventKeys := make(map[string]string, len(events))
	consumedCanonicalKeys := make(map[string]struct{}, len(events))
	for i := range events {
		canonicalKey := canonicalUsageEventKey(events[i])
		incomingKey := strings.TrimSpace(events[i].EventKey)
		if strings.TrimSpace(events[i].RequestID) != "" {
			canonicalEventKeys[canonicalKey] = incomingKey
			continue
		}
		if existingKey := canonicalEventKeys[canonicalKey]; existingKey != "" {
			if incomingKey == canonicalKey {
				events[i].EventKey = existingKey
			} else if existingKey == canonicalKey {
				if _, consumed := consumedCanonicalKeys[canonicalKey]; !consumed {
					events[i].EventKey = existingKey
					consumedCanonicalKeys[canonicalKey] = struct{}{}
				}
			}
			continue
		}

		existing := lookup[canonicalKey]
		existingKey := strings.TrimSpace(existing.EventKey)
		if existingKey == "" {
			canonicalEventKeys[canonicalKey] = incomingKey
			continue
		}
		if incomingKey == canonicalKey {
			events[i].EventKey = existingKey
		} else if existingKey == canonicalKey && !existing.Referenced {
			events[i].EventKey = existingKey
			consumedCanonicalKeys[canonicalKey] = struct{}{}
		}
		canonicalEventKeys[canonicalKey] = existingKey
	}
	return events, nil
}

func loadCanonicalUsageEventLookup(db *gorm.DB, events []entities.UsageEvent) (map[string]canonicalUsageEventLookup, error) {
	canonicalByKey := map[string]entities.UsageEvent{}
	for _, event := range events {
		if strings.TrimSpace(event.RequestID) != "" {
			continue
		}
		canonicalKey := canonicalUsageEventKey(event)
		if _, ok := canonicalByKey[canonicalKey]; !ok {
			canonicalByKey[canonicalKey] = event
		}
	}
	lookup := make(map[string]canonicalUsageEventLookup, len(canonicalByKey))
	if len(canonicalByKey) == 0 {
		return lookup, nil
	}
	canonicalEvents := make([]entities.UsageEvent, 0, len(canonicalByKey))
	for _, event := range canonicalByKey {
		canonicalEvents = append(canonicalEvents, event)
	}
	existingEvents, err := findEquivalentUsageEvents(db, canonicalEvents)
	if err != nil {
		return nil, err
	}
	for _, existing := range existingEvents {
		key := canonicalUsageEventKey(existing)
		if _, ok := lookup[key]; ok {
			continue
		}
		lookup[key] = canonicalUsageEventLookup{EventKey: strings.TrimSpace(existing.EventKey)}
	}
	referenced, err := redisInboxReferencesEventKeys(db, canonicalEventKeys(lookup))
	if err != nil {
		return nil, err
	}
	for key, value := range lookup {
		value.Referenced = referenced[value.EventKey]
		lookup[key] = value
	}
	return lookup, nil
}

func findEquivalentUsageEvents(db *gorm.DB, events []entities.UsageEvent) ([]entities.UsageEvent, error) {
	if len(events) == 0 {
		return nil, nil
	}
	inputs := make([]repository.UsageEventCanonicalLookupInput, 0, len(events))
	for _, event := range events {
		inputs = append(inputs, repository.UsageEventCanonicalLookupInput{
			APIGroupKey:     event.APIGroupKey,
			Model:           event.Model,
			Timestamp:       event.Timestamp,
			Source:          event.Source,
			AuthIndex:       event.AuthIndex,
			Failed:          event.Failed,
			InputTokens:     event.InputTokens,
			OutputTokens:    event.OutputTokens,
			ReasoningTokens: event.ReasoningTokens,
			CachedTokens:    event.CachedTokens,
			TotalTokens:     event.TotalTokens,
		})
	}
	rows, err := repository.FindEquivalentUsageEvents(db, inputs)
	if err != nil {
		return nil, err
	}
	existingEvents := make([]entities.UsageEvent, 0, len(rows))
	for _, row := range rows {
		existingEvents = append(existingEvents, entities.UsageEvent{
			EventKey:        row.EventKey,
			APIGroupKey:     row.APIGroupKey,
			Model:           row.Model,
			Timestamp:       row.Timestamp,
			Source:          row.Source,
			AuthIndex:       row.AuthIndex,
			Failed:          row.Failed,
			InputTokens:     row.InputTokens,
			OutputTokens:    row.OutputTokens,
			ReasoningTokens: row.ReasoningTokens,
			CachedTokens:    row.CachedTokens,
			TotalTokens:     row.TotalTokens,
		})
	}
	return existingEvents, nil
}

func canonicalEventKeys(lookup map[string]canonicalUsageEventLookup) []string {
	keys := make([]string, 0, len(lookup))
	seen := map[string]struct{}{}
	for _, value := range lookup {
		key := strings.TrimSpace(value.EventKey)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func redisInboxReferencesEventKeys(db *gorm.DB, eventKeys []string) (map[string]bool, error) {
	referenced := map[string]bool{}
	if len(eventKeys) == 0 {
		return referenced, nil
	}
	keys, err := repository.ListProcessedRedisUsageInboxEventKeys(db, eventKeys)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		referenced[strings.TrimSpace(key)] = true
	}
	return referenced, nil
}

func canonicalUsageEventKey(event entities.UsageEvent) string {
	return BuildEventKey(
		event.APIGroupKey,
		event.Model,
		event.Timestamp,
		event.Source,
		event.AuthIndex,
		event.Failed,
		repodto.TokenStats{
			InputTokens:     event.InputTokens,
			OutputTokens:    event.OutputTokens,
			ReasoningTokens: event.ReasoningTokens,
			CachedTokens:    event.CachedTokens,
			TotalTokens:     event.TotalTokens,
		},
	)
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
		logrus.WithError(markErr).Warn("failed to mark redis usage inbox process failures")
		return
	}
	for _, row := range rows {
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
