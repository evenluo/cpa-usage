package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"gorm.io/gorm"
)

const redisUsagePerformanceBatchSize = 1_000

var (
	redisUsageBatchBenchmarkResult *entities.UsageEvent
)

// BenchmarkRedisUsageInboxBatchEndToEnd measures the local 1,000-message
// intake and processing path after CPA has returned a batch. Database creation
// and fixture construction are intentionally outside the timed region, so the
// result isolates replay projection, inbox persistence, decoding, event
// persistence, rollup rebuild, and processed-mark work.
func BenchmarkRedisUsageInboxBatchEndToEnd(b *testing.B) {
	for _, benchmarkCase := range []struct {
		name               string
		uniqueRequestIDs   int
		attemptsPerRequest int
	}{
		{name: "unique_requests", uniqueRequestIDs: redisUsagePerformanceBatchSize, attemptsPerRequest: 1},
		{name: "four_attempts_per_request", uniqueRequestIDs: 250, attemptsPerRequest: 4},
	} {
		b.Run(benchmarkCase.name, func(b *testing.B) {
			messages := benchmarkRedisUsageMessages(benchmarkCase.uniqueRequestIDs, benchmarkCase.attemptsPerRequest)
			if len(messages) != redisUsagePerformanceBatchSize {
				b.Fatalf("benchmark fixture must contain %d messages, got %d", redisUsagePerformanceBatchSize, len(messages))
			}
			fetchedAt := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
			totalBytes := 0
			for _, message := range messages {
				totalBytes += len(message)
			}

			b.SetBytes(int64(totalBytes))
			b.ReportAllocs()
			b.ResetTimer()
			b.ReportMetric(redisUsagePerformanceBatchSize, "batch_messages")
			b.ReportMetric(float64(benchmarkCase.uniqueRequestIDs), "unique_request_ids")
			b.ReportMetric(float64(benchmarkCase.attemptsPerRequest), "attempts_per_request")
			for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
				b.StopTimer()
				db := openRedisUsagePerformanceDatabase(b, benchmarkCase.name, benchmarkIndex)
				queue := &redisUsagePerformanceQueue{messages: messages}
				service := NewSyncServiceWithOptions(db, SyncServiceOptions{
					BaseURL:       "https://cpa.example.com",
					RedisQueue:    queue,
					RedisQueueKey: "usage",
					Now:           func() time.Time { return fetchedAt },
				})
				b.StartTimer()

				pullResult, err := service.PullRedisUsageInbox(context.Background())
				if err != nil {
					b.Fatalf("pull benchmark inbox: %v", err)
				}
				processResult, err := service.ProcessRedisUsageInbox(context.Background())
				if err != nil {
					b.Fatalf("process benchmark inbox: %v", err)
				}
				b.StopTimer()

				if queue.calls != 1 || pullResult.InsertedRows != redisUsagePerformanceBatchSize || processResult.InsertedEvents != redisUsagePerformanceBatchSize {
					b.Fatalf("benchmark path did not exercise one complete batch: queue_calls=%d pull=%+v process=%+v", queue.calls, pullResult, processResult)
				}
				if redisUsageAttemptColumnsPresent(db) {
					var failedAttempt redisUsageExpandedBenchmarkRow
					if err := db.Table("usage_events").Where("status_code = ?", 429).First(&failedAttempt).Error; err != nil {
						b.Fatalf("load benchmark failed attempt: %v", err)
					}
					if failedAttempt.ExecutorType == "" || failedAttempt.ReasoningEffort == "" || failedAttempt.ServiceTier == "" || failedAttempt.CacheReadTokens == nil || failedAttempt.CacheCreationTokens == nil || failedAttempt.Source == "" || failedAttempt.AuthIndex == "" || failedAttempt.LatencyMS == 0 {
						b.Fatalf("benchmark path lost expanded attempt fields: %+v", failedAttempt)
					}
				}
				var event entities.UsageEvent
				if err := db.Order("id desc").First(&event).Error; err != nil {
					b.Fatalf("load benchmark event: %v", err)
				}
				redisUsageBatchBenchmarkResult = &event
				b.ReportMetric(float64(redisUsageDatabaseBytes(b, db)), "database_bytes")
				closeRedisUsagePerformanceDatabase(b, db)
			}
		})
	}
}

type redisUsagePerformanceQueue struct {
	calls    int
	messages []string
}

type redisUsageExpandedBenchmarkRow struct {
	StatusCode          int    `gorm:"column:status_code"`
	ExecutorType        string `gorm:"column:executor_type"`
	ReasoningEffort     string `gorm:"column:reasoning_effort"`
	ServiceTier         string `gorm:"column:service_tier"`
	CacheReadTokens     *int64 `gorm:"column:cache_read_tokens"`
	CacheCreationTokens *int64 `gorm:"column:cache_creation_tokens"`
	Source              string `gorm:"column:source"`
	AuthIndex           string `gorm:"column:auth_index"`
	LatencyMS           int64  `gorm:"column:latency_ms"`
}

func redisUsageAttemptColumnsPresent(db *gorm.DB) bool {
	for _, column := range []string{
		"status_code",
		"executor_type",
		"reasoning_effort",
		"service_tier",
		"cache_read_tokens",
		"cache_creation_tokens",
	} {
		if !db.Migrator().HasColumn("usage_events", column) {
			return false
		}
	}
	return true
}

func (q *redisUsagePerformanceQueue) PopUsage(context.Context) ([]string, error) {
	q.calls++
	return q.messages, nil
}

func benchmarkRedisUsageMessages(uniqueRequestIDs int, attemptsPerRequest int) []string {
	count := uniqueRequestIDs * attemptsPerRequest
	messages := make([]string, 0, count)
	for messageIndex := 0; messageIndex < count; messageIndex++ {
		requestIndex := messageIndex / attemptsPerRequest
		failed := "false"
		failure := ""
		if messageIndex%17 == 0 {
			failed = "true"
			failure = `,"fail":{"status_code":429,"body":"excluded failure body"},"response_headers":{"set-cookie":"excluded"}`
		}
		messages = append(messages, fmt.Sprintf(`{"timestamp":"2026-08-31T08:%02d:%02dZ","latency_ms":%d,"ttft_ms":%d,"source":"source-%03d","auth_index":"auth-%04d","provider":"provider-%02d","executor_type":"executor-%d","model":"model-%03d","auth_type":"apikey","api_key":"sk-bench-%04d","request_id":"request-%04d","reasoning_effort":"high","service_tier":"priority","failed":%s%s,"tokens":{"input_tokens":%d,"output_tokens":%d,"reasoning_tokens":%d,"cached_tokens":%d,"cache_read_tokens":%d,"cache_creation_tokens":%d,"total_tokens":%d}}`,
			(messageIndex/60)%60,
			messageIndex%60,
			100+messageIndex%1_000,
			10+messageIndex%100,
			requestIndex%128,
			requestIndex%2_048,
			messageIndex%32,
			messageIndex%4,
			messageIndex%512,
			requestIndex%2_048,
			requestIndex,
			// A repeated request_id is intentionally preserved as separate inbox attempts.
			failed,
			failure,
			100+messageIndex%1_000,
			50+messageIndex%300,
			messageIndex%80,
			messageIndex%100,
			messageIndex%100,
			messageIndex%50,
			150+messageIndex%1_300,
		))
	}
	return messages
}

func openRedisUsagePerformanceDatabase(b *testing.B, benchmarkName string, benchmarkIndex int) *gorm.DB {
	b.Helper()
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(b.TempDir(), fmt.Sprintf("redis-usage-%s-%d.db", benchmarkName, benchmarkIndex))})
	if err != nil {
		b.Fatalf("open redis usage benchmark database: %v", err)
	}
	return db
}

func closeRedisUsagePerformanceDatabase(b *testing.B, db *gorm.DB) {
	b.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("get redis usage benchmark database handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		b.Fatalf("close redis usage benchmark database: %v", err)
	}
}

func redisUsageDatabaseBytes(b *testing.B, db *gorm.DB) int64 {
	b.Helper()
	var pageCount, pageSize int64
	if err := db.Raw("PRAGMA page_count").Scan(&pageCount).Error; err != nil {
		b.Fatalf("read redis usage benchmark page count: %v", err)
	}
	if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		b.Fatalf("read redis usage benchmark page size: %v", err)
	}
	return pageCount * pageSize
}
