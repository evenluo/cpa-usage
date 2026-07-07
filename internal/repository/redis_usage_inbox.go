package repository

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

const (
	RedisUsageInboxStatusPending       = "pending"
	RedisUsageInboxStatusProcessed     = "processed"
	RedisUsageInboxStatusDecodeFailed  = "decode_failed"
	RedisUsageInboxStatusProcessFailed = "process_failed"
	RedisUsageInboxStatusDiscarded     = "discarded"

	redisUsageInboxMaxErrorLength     = 1024
	redisUsageInboxMaxProcessAttempts = 5
)

func InsertRedisUsageInboxMessages(db *gorm.DB, inputs []dto.RedisInboxInsert) ([]entities.RedisUsageInbox, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	rows := make([]entities.RedisUsageInbox, 0, len(inputs))
	// 先把 Redis 原始消息转换成 inbox 行，后续落库只处理标准化后的模型数据。
	for _, input := range inputs {
		hash := sha256.Sum256([]byte(input.RawMessage))
		rows = append(rows, entities.RedisUsageInbox{
			QueueKey:     strings.TrimSpace(input.QueueKey),
			MessageHash:  fmt.Sprintf("%x", hash),
			RawMessage:   input.RawMessage,
			Status:       RedisUsageInboxStatusPending,
			AttemptCount: 0,
			PoppedAt:     input.PoppedAt.UTC(),
		})
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		// Redis 拉取批次仍由配置控制；这里只把数据库写入拆成安全大小。
		return tx.CreateInBatches(&rows, insertBatchSize(entities.RedisUsageInbox{})).Error
	}); err != nil {
		return nil, err
	}
	return rows, nil
}

func MarkRedisUsageInboxProcessed(db *gorm.DB, id uint, eventKey string, processedAt time.Time) error {
	return db.Model(&entities.RedisUsageInbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":          RedisUsageInboxStatusProcessed,
		"usage_event_key": eventKey,
		"processed_at":    processedAt.UTC(),
		"last_error":      "",
	}).Error
}

type RedisUsageInboxProcessedMark struct {
	ID            uint
	UsageEventKey string
}

func MarkRedisUsageInboxProcessedBatch(db *gorm.DB, marks []RedisUsageInboxProcessedMark, processedAt time.Time) error {
	if len(marks) == 0 {
		return nil
	}
	const bindVariablesPerProcessedRow = 3
	maxMarksPerBatch := (sqliteVariableLimit - 3) / bindVariablesPerProcessedRow
	return db.Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(marks); start += maxMarksPerBatch {
			end := min(start+maxMarksPerBatch, len(marks))
			if err := markRedisUsageInboxProcessedBatch(tx, marks[start:end], processedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

func markRedisUsageInboxProcessedBatch(db *gorm.DB, marks []RedisUsageInboxProcessedMark, processedAt time.Time) error {
	caseClauses := make([]string, 0, len(marks))
	idPlaceholders := make([]string, 0, len(marks))
	args := []any{RedisUsageInboxStatusProcessed}
	for _, mark := range marks {
		caseClauses = append(caseClauses, "WHEN ? THEN ?")
		args = append(args, mark.ID, mark.UsageEventKey)
		idPlaceholders = append(idPlaceholders, "?")
	}
	args = append(args, processedAt.UTC())
	args = append(args, processedAt.UTC())
	for _, mark := range marks {
		args = append(args, mark.ID)
	}
	return db.Exec(`
		UPDATE redis_usage_inboxes
		SET status = ?,
		    usage_event_key = CASE id `+strings.Join(caseClauses, " ")+` ELSE usage_event_key END,
		    processed_at = ?,
		    updated_at = ?,
		    last_error = ''
		WHERE id IN (`+strings.Join(idPlaceholders, ", ")+`)`,
		args...,
	).Error
}

func MarkRedisUsageInboxDecodeFailed(db *gorm.DB, id uint, decodeErr error) error {
	return markRedisUsageInboxFailed(db, id, RedisUsageInboxStatusDecodeFailed, decodeErr)
}

func MarkRedisUsageInboxProcessFailed(db *gorm.DB, id uint, processErr error) error {
	return db.Model(&entities.RedisUsageInbox{}).Where("id = ?", id).Updates(map[string]any{
		"status": gorm.Expr(
			"CASE WHEN attempt_count + ? >= ? THEN ? ELSE ? END",
			1,
			redisUsageInboxMaxProcessAttempts,
			RedisUsageInboxStatusDiscarded,
			RedisUsageInboxStatusProcessFailed,
		),
		"attempt_count": gorm.Expr("attempt_count + ?", 1),
		"last_error":    boundedRedisUsageInboxError(processErr),
	}).Error
}

func MarkRedisUsageInboxProcessFailedBatch(db *gorm.DB, ids []uint, processErr error) error {
	if len(ids) == 0 {
		return nil
	}
	maxIDsPerBatch := sqliteVariableLimit - 5
	return db.Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(ids); start += maxIDsPerBatch {
			end := min(start+maxIDsPerBatch, len(ids))
			if err := tx.Model(&entities.RedisUsageInbox{}).Where("id IN ?", ids[start:end]).Updates(map[string]any{
				"status": gorm.Expr(
					"CASE WHEN attempt_count + ? >= ? THEN ? ELSE ? END",
					1,
					redisUsageInboxMaxProcessAttempts,
					RedisUsageInboxStatusDiscarded,
					RedisUsageInboxStatusProcessFailed,
				),
				"attempt_count": gorm.Expr("attempt_count + ?", 1),
				"last_error":    boundedRedisUsageInboxError(processErr),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListProcessedRedisUsageInboxEventKeys(db *gorm.DB, eventKeys []string) ([]string, error) {
	if len(eventKeys) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(eventKeys))
	maxKeysPerBatch := sqliteVariableLimit - 1
	for start := 0; start < len(eventKeys); start += maxKeysPerBatch {
		end := min(start+maxKeysPerBatch, len(eventKeys))
		var keys []string
		if err := db.Model(&entities.RedisUsageInbox{}).
			Where("status = ? AND usage_event_key IN ?", RedisUsageInboxStatusProcessed, eventKeys[start:end]).
			Pluck("usage_event_key", &keys).Error; err != nil {
			return nil, fmt.Errorf("load redis inbox references: %w", err)
		}
		result = append(result, keys...)
	}
	return result, nil
}

// ListProcessableRedisUsageInbox 返回待处理和可重试的数据，不返回已解码失败或已丢弃的数据。
func ListProcessableRedisUsageInbox(db *gorm.DB, limit int) ([]entities.RedisUsageInbox, error) {
	query := `
		SELECT *
		FROM redis_usage_inboxes INDEXED BY idx_redis_usage_inboxes_processable_id
		WHERE status = 'pending' OR status = 'process_failed'
		ORDER BY id ASC`
	args := []any{}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	var rows []entities.RedisUsageInbox
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListPendingRedisUsageInbox(db *gorm.DB, limit int) ([]entities.RedisUsageInbox, error) {
	query := db.Where("status = ?", RedisUsageInboxStatusPending).Order("id asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []entities.RedisUsageInbox
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CleanupRedisUsageInbox 清理已完成和失败的 Redis inbox 原始消息，pending 数据永远不在这里删除。
// processed 保留到下一个本地日开始后才清理；decode_failed/process_failed/discarded 保留 7 天便于排查。
func CleanupRedisUsageInbox(db *gorm.DB, now time.Time) (dto.RedisUsageInboxCleanupResult, error) {
	localNow := now.In(time.Local)
	localDayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.Local)
	processedCutoff := localDayStart.UTC()
	failedCutoff := now.UTC().AddDate(0, 0, -7)
	result := dto.RedisUsageInboxCleanupResult{}

	processedDelete := db.Where("status = ? AND processed_at IS NOT NULL AND processed_at < ?", RedisUsageInboxStatusProcessed, processedCutoff).Delete(&entities.RedisUsageInbox{})
	if processedDelete.Error != nil {
		return result, processedDelete.Error
	}
	result.ProcessedDeleted = processedDelete.RowsAffected

	failedDelete := db.Where("status IN ? AND updated_at < ?", []string{RedisUsageInboxStatusDecodeFailed, RedisUsageInboxStatusProcessFailed, RedisUsageInboxStatusDiscarded}, failedCutoff).Delete(&entities.RedisUsageInbox{})
	if failedDelete.Error != nil {
		return result, failedDelete.Error
	}
	result.FailedDeleted = failedDelete.RowsAffected

	return result, nil
}

func markRedisUsageInboxFailed(db *gorm.DB, id uint, status string, err error) error {
	return db.Model(&entities.RedisUsageInbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"attempt_count": gorm.Expr("attempt_count + ?", 1),
		"last_error":    boundedRedisUsageInboxError(err),
	}).Error
}

func boundedRedisUsageInboxError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) <= redisUsageInboxMaxErrorLength {
		return message
	}
	message = message[:redisUsageInboxMaxErrorLength]
	for !utf8.ValidString(message) {
		message = message[:len(message)-1]
	}
	return message
}
