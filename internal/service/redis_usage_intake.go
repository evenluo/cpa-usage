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
	return newRedisUsageProcessor(i.db).process(i.now().UTC())
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
